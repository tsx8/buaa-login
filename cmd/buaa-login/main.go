package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tsx8/buaa-login/pkg/login"
)

var Version = "dev"

func main() {
	var id, pwd string
	var showVer bool

	flag.StringVar(&id, "i", "", "Student ID")
	flag.StringVar(&pwd, "p", "", "Password")
	flag.BoolVar(&showVer, "v", false, "Show version")
	flag.Parse()

	if showVer {
        fmt.Printf("buaa-login version: %s\n", Version)
        return
    }

	if id == "" || pwd == "" {
		flag.Usage()
		os.Exit(1)
	}

	client := login.New(id, pwd)
	
	success, res, err := client.Run()
	
	if err != nil {
		log.Printf("Login error: %v, Response: %v", err, res)
		os.Exit(1)
	}

	if success {
		fmt.Println("Login successful!")
		printRes(res)
		return
	}

	fmt.Println("Login failed, retrying...")
	for i := range 10 {
		time.Sleep(1 * time.Second)
		success, res, _ = client.Run()
		if success {
			fmt.Printf("Login successful on retry %d!\n", i+1)
			printRes(res)
			return
		}
	}

	fmt.Println("After 10 retries, login failed.")
	printRes(res)
	os.Exit(1)
}

func printRes(res map[string]any) {
	if res == nil {
		return
	}
	for k, v := range res {
		fmt.Printf("%s: %v\n", k, v)
	}
}