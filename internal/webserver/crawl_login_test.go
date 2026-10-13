package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// captureTestLogin runs a login through the fake browser to "captured" and returns its id.
func captureTestLogin(t *testing.T, srv *Server, b *fakeLoginBrowser, rawURL string, cookies ...*http.Cookie) string {
	t.Helper()
	b.mu.Lock()
	b.next = cookies
	b.mu.Unlock()
	rec, started := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": rawURL})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start login: status = %d (body %s)", rec.Code, rec.Body)
	}
	id, _ := started["id"].(string)
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions/"+id+"/capture", nil); rec.Code != http.StatusOK {
		t.Fatalf("capture: status = %d (body %s)", rec.Code, rec.Body)
	}
	return id
}

func TestStartCrawlWithLoginSessionSendsCookiesToSeed(t *testing.T) {
	var mu sync.Mutex
	var got string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			mu.Lock()
			got = r.Header.Get("Cookie")
			mu.Unlock()
		}
		fmt.Fprint(w, `<html><head><title>Members</title></head><body>hi</body></html>`)
	}))
	t.Cleanup(target.Close)

	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	id := captureTestLogin(t, srv, b, target.URL,
		&http.Cookie{Name: "sid", Value: "abc123", Domain: "127.0.0.1"},
		&http.Cookie{Name: "theme", Value: "dark", Domain: "127.0.0.1"},
	)
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "login_session": id})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start crawl: status = %d (body %s)", rec.Code, rec.Body)
	}
	job := pollUntilFinished(t, srv, started["id"].(string))
	if job["status"] != "done" {
		t.Fatalf("job = %v, want done", job)
	}
	mu.Lock()
	defer mu.Unlock()
	if got != "sid=abc123; theme=dark" {
		t.Errorf("Cookie = %q, want %q", got, "sid=abc123; theme=dark")
	}
	if raw, _ := json.Marshal(job); strings.Contains(string(raw), "abc123") {
		t.Error("the job view exposes a cookie value")
	}
}

func TestStartCrawlLoginSessionErrors(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	captured := captureTestLogin(t, srv, b, "https://shop.example.com/",
		&http.Cookie{Name: "sid", Value: "1", Domain: "shop.example.com"})
	rec, waiting := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com/"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("second login: status = %d", rec.Code)
	}
	waitingID, _ := waiting["id"].(string)

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"combined with cookie", map[string]any{"url": "https://shop.example.com/", "login_session": captured, "cookie": "a=b"}, http.StatusBadRequest},
		{"unknown session", map[string]any{"url": "https://shop.example.com/", "login_session": "nope"}, http.StatusNotFound},
		{"not captured yet", map[string]any{"url": "https://shop.example.com/", "login_session": waitingID}, http.StatusConflict},
		{"other host", map[string]any{"url": "https://other.example.com/", "login_session": captured}, http.StatusBadRequest},
		{"headless render", map[string]any{"url": "https://shop.example.com/", "login_session": captured, "render": "headless"}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls", tc.body)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d (body %s)", tc.name, rec.Code, tc.want, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "a=b") {
			t.Errorf("%s: the error echoes the cookie: %s", tc.name, rec.Body)
		}
	}
	if n := len(srv.crawls.list()); n != 0 {
		t.Errorf("%d crawl(s) started despite the errors", n)
	}
}

// A session that was captured and then expired is a 409, not a silent crawl without cookies.
func TestStartCrawlLoginSessionExpiredAndClosed(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	id := captureTestLogin(t, srv, b, "https://shop.example.com/",
		&http.Cookie{Name: "sid", Value: "1", Domain: "shop.example.com"})
	srv.logins.expire(id)
	rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": "https://shop.example.com/", "login_session": id})
	if rec.Code != http.StatusConflict {
		t.Errorf("expired session: status = %d, want 409 (body %s)", rec.Code, rec.Body)
	}
	if n := len(srv.crawls.list()); n != 0 {
		t.Errorf("%d crawl(s) started despite the error", n)
	}
}

// An internationalized host: the user types the Unicode form, Chromium stores cookie domains
// in the ASCII (punycode) form. Both sides compare in ASCII, so the capture finds the cookie
// and the crawl accepts the session for the Unicode seed.
func TestLoginSessionMatchesIDNHosts(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	id := captureTestLogin(t, srv, b, "https://bücher.example/",
		&http.Cookie{Name: "sid", Value: "1", Domain: "xn--bcher-kva.example"})
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": "https://bücher.example/", "login_session": id, "max_pages": 1})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start crawl: status = %d, want 202 (body %s)", rec.Code, rec.Body)
	}
	jobID := started["id"].(string)
	doJSON(t, srv, http.MethodPost, "/api/crawls/"+jobID+"/cancel", nil)
	pollUntilFinished(t, srv, jobID)
}

// The seed's port is not part of the host a cookie belongs to: a session logged in on
// "shop.example:8443" is for host "shop.example", and a seed with another port still matches.
func TestLoginSessionIgnoresSeedPort(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	id := captureTestLogin(t, srv, b, "https://shop.example:8443/",
		&http.Cookie{Name: "sid", Value: "1", Domain: "shop.example"})
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": "https://shop.example:9443/", "login_session": id, "max_pages": 1})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start crawl: status = %d, want 202 (body %s)", rec.Code, rec.Body)
	}
	jobID := started["id"].(string)
	doJSON(t, srv, http.MethodPost, "/api/crawls/"+jobID+"/cancel", nil)
	pollUntilFinished(t, srv, jobID)
}

func TestAsciiHost(t *testing.T) {
	for in, want := range map[string]string{
		"Shop.Example.COM":  "shop.example.com",
		"bücher.example":    "xn--bcher-kva.example",
		"my_shop.example":   "my_shop.example",
		"xn--bcher-kva.com": "xn--bcher-kva.com",
	} {
		if got := asciiHost(in); got != want {
			t.Errorf("asciiHost(%q) = %q, want %q", in, got, want)
		}
	}
}
