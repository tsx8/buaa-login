package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tsx8/buaa-login/pkg/login"
)

type stubLoginRunner struct {
	err   error
	calls int
}

func (s *stubLoginRunner) Run() error {
	s.calls++
	return s.err
}

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

func TestRunReturnsStableExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		maxRetry   string
		wantCode   int
		wantCalls  int
		wantSleeps int
	}{
		{name: "success", wantCode: exitSuccess, wantCalls: 1},
		{
			name:      "authentication rejection",
			err:       &login.Error{Kind: login.ErrorAuthentication, Operation: "login", Code: "password_error", Message: "rejected"},
			maxRetry:  "3",
			wantCode:  exitAuthentication,
			wantCalls: 1,
		},
		{
			name:       "transient exhaustion",
			err:        &login.Error{Kind: login.ErrorTransient, Operation: "login", Message: "temporary failure"},
			maxRetry:   "2",
			wantCode:   exitTransient,
			wantCalls:  3,
			wantSleeps: 2,
		},
		{
			name:      "configuration failure",
			err:       &login.Error{Kind: login.ErrorConfiguration, Operation: "login", Message: "invalid configuration"},
			maxRetry:  "3",
			wantCode:  exitUsage,
			wantCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &stubLoginRunner{err: test.err}
			var stdout, stderr bytes.Buffer
			sleeps := 0
			args := []string{"-i", "Student", "-p", "not-logged", "--interface", "campus0"}
			if test.maxRetry != "" {
				args = append(args, "-r", test.maxRetry)
			}
			code := run(args, &stdout, &stderr, func(config login.Config) (loginRunner, error) {
				if config.StudentID != "student" || config.Password != "not-logged" || config.Interface != "campus0" {
					t.Fatalf("client config = %#v, want normalized credentials and campus0", config)
				}
				return runner, nil
			}, func(time.Duration) { sleeps++ })

			if code != test.wantCode || runner.calls != test.wantCalls || sleeps != test.wantSleeps {
				t.Fatalf("run() = code %d, calls %d, sleeps %d; want %d, %d, %d", code, runner.calls, sleeps, test.wantCode, test.wantCalls, test.wantSleeps)
			}
			if bytes.Contains(stderr.Bytes(), []byte("not-logged")) {
				t.Fatal("stderr contains password")
			}
		})
	}
}

func TestRunRejectsInvalidCLIConfiguration(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"-i", "student", "-p", "secret", "-r", "-1"},
		{"-i", "student", "-p", "secret", "unexpected"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr, func(login.Config) (loginRunner, error) {
			t.Fatal("client must not be created for invalid CLI configuration")
			return nil, nil
		}, func(time.Duration) {})
		if code != exitUsage {
			t.Fatalf("run(%q) = %d, want %d", args, code, exitUsage)
		}
	}
}

func TestRunReturnsFactoryErrorWithoutRetry(t *testing.T) {
	var stdout, stderr bytes.Buffer
	factoryCalls := 0
	sleeps := 0
	code := run(
		[]string{"-i", "student", "-p", "not-logged", "--interface", "missing0", "-r", "3"},
		&stdout,
		&stderr,
		func(config login.Config) (loginRunner, error) {
			factoryCalls++
			if config.Interface != "missing0" {
				t.Fatalf("interface = %q, want missing0", config.Interface)
			}
			return nil, &login.Error{
				Kind:      login.ErrorConfiguration,
				Operation: "resolve network interface",
				Message:   "interface does not exist",
			}
		},
		func(time.Duration) { sleeps++ },
	)

	if code != exitUsage || factoryCalls != 1 || sleeps != 0 {
		t.Fatalf("run() = code %d, factory calls %d, sleeps %d; want %d, 1, 0", code, factoryCalls, sleeps, exitUsage)
	}
}
