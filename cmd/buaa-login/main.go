package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tsx8/buaa-login/pkg/login"
)

var Version = "dev"

var unquotedCredentialKey = regexp.MustCompile(`([,{]\s*)(stuid|paswd)\s*:`)

func main() {
	var id, pwd, credentialsFile string
	var maxRetry int
	var showVer bool

	flag.StringVar(&id, "i", "", "Student ID")
	flag.StringVar(&pwd, "p", "", "Password")
	flag.StringVar(&credentialsFile, "credentials-file", "", "Path to a JSON credentials file")
	flag.IntVar(&maxRetry, "r", 0, "Max retry times (default 0)")
	flag.BoolVar(&showVer, "v", false, "Show version")
	flag.Parse()

	if showVer {
		fmt.Printf("buaa-login version: %s\n", getVersion())
		return
	}

	if credentialsFile != "" {
		if id != "" || pwd != "" {
			fmt.Fprintln(os.Stderr, "credentials-file cannot be combined with -i or -p")
			os.Exit(2)
		}
		var err error
		id, pwd, err = readCredentials(credentialsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read credentials: %v\n", err)
			os.Exit(2)
		}
	}

	if id == "" || pwd == "" {
		flag.Usage()
		os.Exit(1)
	}

	id = strings.ToLower(strings.TrimSpace(id))
	client := login.New(id, pwd)
	totalAttempts := 1 + maxRetry

	for i := range totalAttempts {
		if i > 0 {
			fmt.Printf("Retry attempt %d/%d after 2 seconds...\n", i, maxRetry)
			time.Sleep(2 * time.Second)
		}

		success, res, err := client.Run()

		if err != nil {
			log.Printf("Attempt %d error: %v", i+1, err)
			continue
		}

		if success {
			printRes(res)
			fmt.Println("Login successful!")
			os.Exit(0)
		}

		printRes(res)
		if errMsg, ok := res["error"].(string); ok {
			log.Printf("Login failed: %s", errMsg)
		} else {
			log.Printf("Login failed (unknown error)")
		}
	}

	fmt.Println("All attempts failed.")
	os.Exit(1)
}

func readCredentials(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	var credentials struct {
		ID       string `json:"stuid"`
		Password string `json:"paswd"`
	}
	if err := json.Unmarshal(data, &credentials); err != nil {
		normalized := unquotedCredentialKey.ReplaceAll(data, []byte(`${1}"${2}":`))
		if normalizedErr := json.Unmarshal(normalized, &credentials); normalizedErr != nil {
			return "", "", fmt.Errorf("invalid JSON: %w", err)
		}
	}
	if strings.TrimSpace(credentials.ID) == "" || credentials.Password == "" {
		return "", "", fmt.Errorf("stuid and paswd must both be non-empty")
	}
	return credentials.ID, credentials.Password, nil
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

func printRes(res map[string]any) {
	if res == nil {
		return
	}
	for k, v := range res {
		fmt.Printf("%s: %v\n", k, v)
	}
}
