package login

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("forced transport failure")
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	client, err := New(Config{StudentID: "student", Password: "secret"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}

func newPortalServer(t *testing.T, loginResponse string) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var challengeRequests atomic.Int32
	var loginRequests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.Redirect(w, r, "/index_1.html", http.StatusFound)
		case "/index_1.html":
			fmt.Fprint(w, `<meta http-equiv="refresh" content="0;url=/srun_portal_pc?ac_id=78&amp;theme=buaa">`)
		case "/srun_portal_pc":
			if got := r.URL.Query().Get("ac_id"); got != "78" {
				t.Errorf("portal ac_id = %q, want 78", got)
			}
			if got := r.URL.Query().Get("theme"); got != "buaa" {
				t.Errorf("portal theme = %q, want buaa", got)
			}
			fmt.Fprint(w, `var CONFIG = { ip: "10.0.0.8", acid: "78" };`)
		case "/cgi-bin/get_challenge":
			challengeRequests.Add(1)
			if got := r.URL.Query().Get("username"); got != "student" {
				t.Errorf("challenge username = %q, want student", got)
			}
			if got := r.URL.Query().Get("ip"); got != "10.0.0.8" {
				t.Errorf("challenge ip = %q, want 10.0.0.8", got)
			}
			fmt.Fprint(w, `jQuery({"challenge":"test-token"})`)
		case "/cgi-bin/srun_portal":
			loginRequests.Add(1)
			query := r.URL.Query()
			for key, want := range map[string]string{
				"action": "login",
				"ac_id":  "78",
				"ip":     "10.0.0.8",
				"n":      "200",
				"type":   "1",
			} {
				if got := query.Get(key); got != want {
					t.Errorf("login %s = %q, want %q", key, got, want)
				}
			}
			if query.Get("chksum") == "" || query.Get("info") == "" {
				t.Error("login request is missing chksum or info")
			}
			fmt.Fprint(w, loginResponse)
		default:
			http.NotFound(w, r)
		}
	}))
	return server, &challengeRequests, &loginRequests
}

func TestRunUsesNewPortalDiscoveryFlow(t *testing.T) {
	server, challengeRequests, loginRequests := newPortalServer(t, `jQuery({"error":"ok"})`)
	defer server.Close()

	client := newTestClient(t)
	client.baseURL = server.URL

	if err := client.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if challengeRequests.Load() != 1 || loginRequests.Load() != 1 {
		t.Fatalf("request counts: challenge=%d login=%d, want 1 each", challengeRequests.Load(), loginRequests.Load())
	}
}

func TestRunTreatsAlreadyOnlineResponseAsSuccess(t *testing.T) {
	server, _, _ := newPortalServer(t, `jQuery({"error":"ok","res":"ok","suc_msg":"ip_already_online_error","ecode":0})`)
	defer server.Close()

	client := newTestClient(t)
	client.baseURL = server.URL

	if err := client.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunClassifiesGatewayFailures(t *testing.T) {
	tests := []struct {
		name     string
		response string
		kind     ErrorKind
		code     string
	}{
		{name: "expired challenge", response: `jQuery({"res":"challenge_expire_error"})`, kind: ErrorTransient, code: "challenge_expire_error"},
		{name: "signature error", response: `jQuery({"res":"sign_error"})`, kind: ErrorTransient, code: "sign_error"},
		{name: "wrong password", response: `jQuery({"error":"password_error"})`, kind: ErrorAuthentication, code: "password_error"},
		{name: "unknown rejection", response: `jQuery({"error":"fail"})`, kind: ErrorAuthentication, code: "fail"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, _, _ := newPortalServer(t, test.response)
			defer server.Close()

			client := newTestClient(t)
			client.baseURL = server.URL
			err := client.Run()
			if err == nil {
				t.Fatal("Run() error = nil, want classified failure")
			}
			var classified *Error
			if !errors.As(err, &classified) {
				t.Fatalf("Run() error type = %T, want *Error", err)
			}
			if classified.Kind != test.kind || classified.Code != test.code {
				t.Fatalf("Run() error = kind %v code %q, want kind %v code %q", classified.Kind, classified.Code, test.kind, test.code)
			}
		})
	}
}

func TestNewClientVerifiesGatewayCertificate(t *testing.T) {
	client := newTestClient(t)
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil {
		t.Fatal("New() transport has no TLS configuration")
	}
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("New() disables TLS certificate verification")
	}
	if got := transport.TLSClientConfig.ServerName; got != gatewayTLSName {
		t.Fatalf("TLS ServerName = %q, want %q", got, gatewayTLSName)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `ac_id=78`)
	}))
	defer server.Close()
	client.baseURL = server.URL

	if err := client.Run(); err == nil || KindOf(err) != ErrorTransient {
		t.Fatalf("Run() with untrusted certificate error = %v, want transient verification failure", err)
	}
}

func TestRunRejectsMissingCredentials(t *testing.T) {
	_, err := New(Config{})
	if err == nil || KindOf(err) != ErrorConfiguration {
		t.Fatalf("New() error = %v, want configuration error", err)
	}
}

func TestRequestErrorsDoNotExposeQueryValues(t *testing.T) {
	client := newTestClient(t)
	client.httpClient.Transport = failingTransport{}

	_, err := client.get("https://example.invalid/login", url.Values{
		"username": {"student"},
		"password": {"secret"},
	})
	if err == nil {
		t.Fatal("get() error = nil, want transport failure")
	}
	if got := err.Error(); got != "forced transport failure" {
		t.Fatalf("get() error = %q, want URL-free transport error", got)
	}
}

func TestGatewayResultCodeRejectsUnsafeValues(t *testing.T) {
	for _, code := range []string{"password_error\nsecret", "password error", string(make([]byte, 65))} {
		if got := gatewayResultCode(map[string]interface{}{"error": code}); got != "unknown" {
			t.Fatalf("gatewayResultCode(%q) = %q, want unknown", code, got)
		}
	}
}

func TestClassifiedErrorFormattingAndWrapping(t *testing.T) {
	classified := &Error{
		Kind:      ErrorAuthentication,
		Operation: "login",
		Code:      "password_error",
		Message:   "rejected",
	}
	if got, want := classified.Error(), "login: rejected (code password_error)"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if got := KindOf(fmt.Errorf("outer: %w", classified)); got != ErrorAuthentication {
		t.Fatalf("KindOf(wrapped error) = %v, want %v", got, ErrorAuthentication)
	}

	cause := errors.New("network unavailable")
	transient := transientError("request challenge", cause)
	if !errors.Is(transient, cause) {
		t.Fatal("transient error does not unwrap to its cause")
	}
	if got, want := transient.Error(), "request challenge: temporary failure: network unavailable"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
