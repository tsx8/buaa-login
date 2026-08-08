package login

import (
	"errors"
	"net"
	"net/url"
	"testing"
)

func activeLoopbackInterface(t *testing.T) *net.Interface {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for i := range interfaces {
		if interfaces[i].Flags&net.FlagLoopback != 0 && interfaces[i].Flags&net.FlagUp != 0 {
			return &interfaces[i]
		}
	}
	t.Fatal("no active loopback interface found")
	return nil
}

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
