package webserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/render"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

// openedURLs returns the URLs the browser was asked to open, in order.
func (b *fakeLoginBrowser) openedURLs() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.opened...)
}

func newLoginServer(t *testing.T, b *fakeLoginBrowser) *Server {
	t.Helper()
	srv := New(store.New(t.TempDir()), WithLoginBrowser(b))
	t.Cleanup(srv.Close)
	return srv
}

func loginRaw(t *testing.T, srv *Server, method, path, contentType, origin, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "localhost:8080"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestLoginAvailability(t *testing.T) {
	_, out := doJSON(t, newLoginServer(t, &fakeLoginBrowser{}), http.MethodGet, "/api/login-sessions/available", nil)
	if out["available"] != true || out["reason"] != nil {
		t.Errorf("available = %v", out)
	}
	_, out = doJSON(t, newLoginServer(t, &fakeLoginBrowser{available: render.ErrNoBrowser}), http.MethodGet, "/api/login-sessions/available", nil)
	if out["available"] != false {
		t.Errorf("available = %v, want false", out["available"])
	}
	if reason, _ := out["reason"].(string); !strings.Contains(reason, "Chromium") {
		t.Errorf("reason = %q, want it to mention Chromium", reason)
	}
}

func TestLoginSessionFlow(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	rec, started := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "shop.example.com/account"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start: status = %d (body %s)", rec.Code, rec.Body)
	}
	if started["state"] != "waiting" || started["host"] != "shop.example.com" {
		t.Errorf("start view = %v", started)
	}
	if got := b.openedURLs(); !reflect.DeepEqual(got, []string{"https://shop.example.com/account"}) {
		t.Errorf("opened %v, want the https seed URL (is WithLoginBrowser applied after New builds s.logins?)", got)
	}
	id, _ := started["id"].(string)

	b.window(t, 0).setCookies(
		&http.Cookie{Name: "sid", Value: "s3cret-value", Domain: "shop.example.com"},
		&http.Cookie{Name: "idp", Value: "other-secret", Domain: "idp.example"},
	)
	rec, captured := doJSON(t, srv, http.MethodPost, "/api/login-sessions/"+id+"/capture", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("capture: status = %d (body %s)", rec.Code, rec.Body)
	}
	if captured["state"] != "captured" || captured["cookie_count"] != float64(1) || !reflect.DeepEqual(captured["cookie_names"], []any{"sid"}) {
		t.Errorf("captured view = %v", captured)
	}
	if strings.Contains(rec.Body.String(), "s3cret-value") || strings.Contains(rec.Body.String(), "other-secret") {
		t.Errorf("capture response leaks a cookie value: %s", rec.Body)
	}

	rec, got := doJSON(t, srv, http.MethodGet, "/api/login-sessions/"+id, nil)
	if rec.Code != http.StatusOK || got["state"] != "captured" || strings.Contains(rec.Body.String(), "s3cret-value") {
		t.Errorf("get: status %d, body %s", rec.Code, rec.Body)
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions/"+id+"/capture", nil); rec.Code != http.StatusConflict {
		t.Errorf("second capture: status = %d, want 409", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodDelete, "/api/login-sessions/"+id, nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete: status = %d, want 204", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodGet, "/api/login-sessions/"+id, nil); rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: status = %d, want 404", rec.Code)
	}
}

func TestStartLoginValidatesAndStripsCredentials(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	for _, bad := range []string{"", "ftp://example.com/", "https://"} {
		if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": bad}); rec.Code != http.StatusBadRequest {
			t.Errorf("url %q: status = %d, want 400", bad, rec.Code)
		}
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://alice:pw@shop.example.com/"}); rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if got := b.openedURLs(); !reflect.DeepEqual(got, []string{"https://shop.example.com/"}) {
		t.Errorf("opened %v, want only the credential-free URL", got)
	}
}

func TestStartLoginErrorsDoNotEchoCredentials(t *testing.T) {
	srv := newLoginServer(t, &fakeLoginBrowser{})
	rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "ftp://alice:hunter2@example.com/"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "hunter2") {
		t.Errorf("error leaks the URL's password: %s", rec.Body)
	}
}

func TestStartLoginGuards(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	if rec := loginRaw(t, srv, http.MethodPost, "/api/login-sessions", "text/plain", "", `{"url":"https://shop.example.com"}`); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: status = %d, want 415", rec.Code)
	}
	if rec := loginRaw(t, srv, http.MethodPost, "/api/login-sessions", "application/json", "http://evil.example", `{"url":"https://shop.example.com"}`); rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin: status = %d, want 403", rec.Code)
	}
	if got := b.openedURLs(); len(got) != 0 {
		t.Errorf("rejected requests opened windows: %v", got)
	}
}

func TestStartLoginWhenBrowserFails(t *testing.T) {
	rec, out := doJSON(t, newLoginServer(t, &fakeLoginBrowser{err: render.ErrNoBrowser}), http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("no browser: status = %d, want 503", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(strings.ToLower(msg), "no chromium-class browser found") {
		t.Errorf("no browser: error = %q", msg)
	}
	rec, out = doJSON(t, newLoginServer(t, &fakeLoginBrowser{err: errors.New("boom")}), http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("launch failure: status = %d, want 503", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "boom") {
		t.Errorf("launch failure: error = %q", msg)
	}
}

func TestLoginSessionWindowClosedByUser(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := newLoginServer(t, b)
	_, started := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"})
	id, _ := started["id"].(string)
	b.window(t, 0).userCloses()
	waitForLogin(t, "state closed", func() bool {
		_, got := doJSON(t, srv, http.MethodGet, "/api/login-sessions/"+id, nil)
		return got["state"] == "closed"
	})
	rec, out := doJSON(t, srv, http.MethodPost, "/api/login-sessions/"+id+"/capture", nil)
	if rec.Code != http.StatusConflict {
		t.Errorf("capture: status = %d, want 409", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "closed") {
		t.Errorf("capture: error = %q, want it to say the window was closed", msg)
	}
}

func TestLoginRoutesUnknownSession(t *testing.T) {
	srv := newLoginServer(t, &fakeLoginBrowser{})
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/login-sessions/nope"},
		{http.MethodPost, "/api/login-sessions/nope/capture"},
		{http.MethodDelete, "/api/login-sessions/nope"},
	} {
		if rec, _ := doJSON(t, srv, tc.method, tc.path, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
}

func TestLoginWindowLimitReturns429(t *testing.T) {
	srv := newLoginServer(t, &fakeLoginBrowser{})
	for i := 0; i < maxOpenLoginWindows; i++ {
		if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"}); rec.Code != http.StatusAccepted {
			t.Fatalf("start %d: status = %d", i, rec.Code)
		}
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("start past the cap: status = %d, want 429", rec.Code)
	}
}

func TestServerCloseClosesLoginWindows(t *testing.T) {
	b := &fakeLoginBrowser{}
	srv := New(store.New(t.TempDir()), WithLoginBrowser(b))
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"}); rec.Code != http.StatusAccepted {
		t.Fatalf("start: status = %d", rec.Code)
	}
	srv.Close()
	if !b.window(t, 0).isClosed() {
		t.Error("Close left a login window open")
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/login-sessions", map[string]string{"url": "https://shop.example.com"}); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("start after Close: status = %d, want 503", rec.Code)
	}
	if got := b.openedURLs(); len(got) != 1 {
		t.Errorf("a window opened after Close: %v", got)
	}
}

func TestSweepStaleLoginProfiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	mk := func(name string, age time.Duration, isDir bool) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if isDir {
			if err := os.MkdirAll(filepath.Join(p, "Default"), 0o700); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(-age)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
		return p
	}
	oldProfile := mk("gocrawl-login-111", 2*time.Hour, true)
	freshProfile := mk("gocrawl-login-222", 10*time.Minute, true)
	oldOther := mk("other-dir", 5*time.Hour, true)
	oldLookalike := mk("not-gocrawl-login-333", 5*time.Hour, true)
	oldFile := mk("gocrawl-login-file", 5*time.Hour, false)

	SweepStaleLoginProfiles(dir, time.Hour)

	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }
	if exists(oldProfile) {
		t.Error("stale login profile was not removed")
	}
	for name, p := range map[string]string{"fresh profile": freshProfile, "other directory": oldOther, "lookalike": oldLookalike, "plain file": oldFile} {
		if !exists(p) {
			t.Errorf("%s was removed", name)
		}
	}
	// A missing directory is not an error.
	SweepStaleLoginProfiles(filepath.Join(dir, "missing"), time.Hour)
}

// A prefixed symlink to a directory elsewhere is not a login profile: the sweep leaves both
// the link and its target alone. A negative maxAge makes every entry count as stale, so only
// the entry's type can save the link (os.Chtimes follows symlinks, so a link can't be aged).
func TestSweepStaleLoginProfilesLeavesSymlinks(t *testing.T) {
	dir := t.TempDir()
	outsideDir := t.TempDir()
	keep := filepath.Join(outsideDir, "precious.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, render.LoginProfilePrefix+"link")
	if err := os.Symlink(outsideDir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	realProfile := filepath.Join(dir, render.LoginProfilePrefix+"real")
	if err := os.Mkdir(realProfile, 0o700); err != nil {
		t.Fatal(err)
	}

	SweepStaleLoginProfiles(dir, -time.Hour)

	if _, err := os.Lstat(link); err != nil {
		t.Errorf("the symlink was removed: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("the symlink target was touched: %v", err)
	}
	if _, err := os.Stat(realProfile); err == nil {
		t.Error("a real stale profile next to the symlink was not removed")
	}
}
