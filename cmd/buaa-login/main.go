package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tsx8/buaa-login/pkg/login"
)

var Version = "dev"

func main() {
	var id, pwd string
	var maxRetry int
	var showVer bool

	flag.StringVar(&id, "i", "", "Student ID")
	flag.StringVar(&pwd, "p", "", "Password")
	flag.IntVar(&maxRetry, "r", 0, "Max retry times (default 0)")
	flag.BoolVar(&showVer, "v", false, "Show version")
	flag.Parse()

	if showVer {
		fmt.Printf("buaa-login version: %s\n", getVersion())
		return
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
		} else if success {
			printRes(res)
			fmt.Println("Login successful!")
			os.Exit(0)
		} else {
			printRes(res)
			log.Printf("Login failed (Server returned failure).")
		}
	}

	fmt.Println("All attempts failed.")
	os.Exit(1)
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
