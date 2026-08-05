package login

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRunUsesNewPortalDiscoveryFlow(t *testing.T) {
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
			fmt.Fprint(w, `jQuery({"error":"ok"})`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("student", "secret")
	client.BaseURL = server.URL

	success, result, err := client.Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !success {
		t.Fatalf("Run() success = false, result = %#v", result)
	}
	if challengeRequests.Load() != 1 || loginRequests.Load() != 1 {
		t.Fatalf("request counts: challenge=%d login=%d, want 1 each", challengeRequests.Load(), loginRequests.Load())
	}
}
