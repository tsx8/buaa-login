package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tsx8/buaa-login/pkg/login"
)

var Version = "dev"

const (
	exitSuccess        = 0
	exitTransient      = 1
	exitUsage          = 2
	exitAuthentication = 3
)

type loginRunner interface {
	Run() error
}

type clientFactory func(login.Config) (loginRunner, error)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, func(config login.Config) (loginRunner, error) {
		return login.New(config)
	}, time.Sleep))
}

func run(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	newClient clientFactory,
	sleep func(time.Duration),
) int {
	fs := flag.NewFlagSet("buaa-login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var id, password, credentialsFile, interfaceName string
	var maxRetry int
	var showVersion bool

	fs.StringVar(&id, "i", "", "Student ID")
	fs.StringVar(&password, "p", "", "Password")
	fs.StringVar(&credentialsFile, "credentials-file", "", "Path to a JSON credentials file")
	fs.StringVar(&interfaceName, "interface", "", "Network interface used for gateway requests")
	fs.IntVar(&maxRetry, "r", 0, "Max retry times (default 0)")
	fs.BoolVar(&showVersion, "v", false, "Show version")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments")
		return exitUsage
	}
	if showVersion {
		fmt.Fprintf(stdout, "buaa-login version: %s\n", getVersion())
		return exitSuccess
	}
	if maxRetry < 0 {
		fmt.Fprintln(stderr, "-r must be zero or greater")
		return exitUsage
	}

	if credentialsFile != "" {
		if id != "" || password != "" {
			fmt.Fprintln(stderr, "credentials-file cannot be combined with -i or -p")
			return exitUsage
		}
		var err error
		id, password, err = readCredentials(credentialsFile)
		if err != nil {
			fmt.Fprintf(stderr, "failed to read credentials: %v\n", err)
			return exitUsage
		}
	}

	if id == "" || password == "" {
		fs.Usage()
		return exitUsage
	}

	id = strings.ToLower(strings.TrimSpace(id))
	client, err := newClient(login.Config{
		StudentID: id,
		Password:  password,
		Interface: interfaceName,
	})
	if err != nil {
		fmt.Fprintf(stderr, "Failed to configure login: %v\n", err)
		return exitCodeForError(err)
	}
	for attempt := 0; attempt <= maxRetry; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(stderr, "Retry attempt %d/%d after 2 seconds...\n", attempt, maxRetry)
			sleep(2 * time.Second)
		}

		err := client.Run()
		if err == nil {
			fmt.Fprintln(stdout, "Login successful!")
			return exitSuccess
		}

		exitCode := exitCodeForError(err)
		fmt.Fprintf(stderr, "Attempt %d failed: %v\n", attempt+1, err)
		if exitCode != exitTransient {
			return exitCode
		}
	}

	fmt.Fprintln(stderr, "All attempts failed.")
	return exitTransient
}

func exitCodeForError(err error) int {
	switch login.KindOf(err) {
	case login.ErrorAuthentication:
		return exitAuthentication
	case login.ErrorConfiguration:
		return exitUsage
	default:
		return exitTransient
	}
}

func readCredentials(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	var credentials struct {
		StudentID string `json:"student_id"`
		Password  string `json:"password"`
	}
	if err := json.Unmarshal(data, &credentials); err != nil {
		return "", "", fmt.Errorf("invalid JSON: %w", err)
	}
	if strings.TrimSpace(credentials.StudentID) == "" || credentials.Password == "" {
		return "", "", fmt.Errorf("student_id and password must both be non-empty")
	}
	return credentials.StudentID, credentials.Password, nil
}

func getVersion() string {
	if Version != "dev" {
		return Version
	}

	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return Version
}
