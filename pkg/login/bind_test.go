package login

import (
	"errors"
	"net"
	"net/url"
	"testing"
)

func TestNewRejectsUnknownInterface(t *testing.T) {
	_, err := New(Config{
		StudentID: "student",
		Password:  "secret",
		Interface: "buaa-login-interface-that-does-not-exist",
	})
	if err == nil || KindOf(err) != ErrorConfiguration {
		t.Fatalf("New() error = %v, want configuration error", err)
	}
}

func TestRequestErrorPreservesClassification(t *testing.T) {
	classified := &Error{
		Kind:      ErrorConfiguration,
		Operation: "bind network interface",
		Message:   "unable to bind socket",
	}
	wrapper := &url.Error{
		Op:  "Get",
		URL: "https://example.invalid",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: classified},
	}

	err := requestError("initialize gateway session", wrapper)
	var got *Error
	if !errors.As(err, &got) || got != classified || KindOf(err) != ErrorConfiguration {
		t.Fatalf("requestError() = %#v, want original configuration error", err)
	}
}
