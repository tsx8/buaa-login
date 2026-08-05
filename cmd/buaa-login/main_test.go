package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCredentials(t *testing.T) {
	for name, content := range map[string]string{
		"JSON":      `{"stuid":"student","paswd":"secret with spaces"}`,
		"bare keys": `{stuid:"student",paswd:"secret with spaces"}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "credentials.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}

			id, password, err := readCredentials(path)
			if err != nil {
				t.Fatalf("readCredentials() error = %v", err)
			}
			if id != "student" || password != "secret with spaces" {
				t.Fatalf("readCredentials() = (%q, %q), want student and preserved password", id, password)
			}
		})
	}
}

func TestReadCredentialsRejectsIncompleteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(path, []byte(`{"stuid":"student"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := readCredentials(path); err == nil {
		t.Fatal("readCredentials() error = nil, want incomplete credentials error")
	}
}
