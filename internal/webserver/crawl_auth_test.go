package webserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

func TestStartCrawlRejectsBearerTokenConflicts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := config.SaveHeaderProfile("auth", "Authorization: Basic eDp5\n"); err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t)
	const token = "tok-secret"
	cases := map[string]map[string]any{
		"basic_auth":     {"url": "http://127.0.0.1:1/", "bearer_token": token, "basic_auth": "alice:pw"},
		"seed userinfo":  {"url": "http://alice:pw@127.0.0.1:1/", "bearer_token": token},
		"inline header":  {"url": "http://127.0.0.1:1/", "bearer_token": token, "headers": []string{"Authorization: Token abc"}},
		"header profile": {"url": "http://127.0.0.1:1/", "bearer_token": token, "header_profile": "auth"},
	}
	for name, body := range cases {
		rec, out := doJSON(t, srv, http.MethodPost, "/api/crawls", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (body %s)", name, rec.Code, rec.Body)
			continue
		}
		if msg, _ := out["error"].(string); !strings.Contains(msg, "bearer_token") {
			t.Errorf("%s: error %q doesn't name bearer_token", name, msg)
		}
		if strings.Contains(rec.Body.String(), token) {
			t.Errorf("%s: response leaks the token: %s", name, rec.Body)
		}
	}
	if n := len(srv.crawls.list()); n != 0 {
		t.Errorf("%d crawl(s) started despite the conflicts", n)
	}
}

// End to end: the token reaches the seed as a Bearer Authorization header.
func TestStartCrawlSendsBearerTokenToSeed(t *testing.T) {
	var mu sync.Mutex
	var got string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			mu.Lock()
			got = r.Header.Get("Authorization")
			mu.Unlock()
		}
		fmt.Fprint(w, `<html><head><title>Home</title></head><body>ok</body></html>`)
	}))
	t.Cleanup(target.Close)

	srv := newTestServer(t)
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "bearer_token": "tok-123"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %s)", rec.Code, rec.Body)
	}
	job := pollUntilFinished(t, srv, started["id"].(string))
	if job["status"] != "done" {
		t.Fatalf("job = %v, want done", job)
	}
	mu.Lock()
	defer mu.Unlock()
	if got != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer tok-123")
	}
}
