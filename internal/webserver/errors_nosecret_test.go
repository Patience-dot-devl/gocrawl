package webserver

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

// A seed url.Parse rejects must not be echoed: its userinfo may hold a password.
func TestStartCrawlUnparsableSeedDoesNotEchoPassword(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": "https://alice:s3cret@example.com/%zz"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "s3cret") {
		t.Errorf("response leaks the seed password: %s", rec.Body)
	}
	if n := len(srv.crawls.list()); n != 0 {
		t.Errorf("%d crawl(s) started for an unparsable seed", n)
	}
}

// A header profile deleted after the form loaded must give a plain "not found", not the
// OS error with the profile's path.
func TestStartCrawlMissingHeaderProfileHasNoPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads this on Windows
	srv := newTestServer(t)
	rec, out := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": "http://127.0.0.1:1/", "header_profile": "gone"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	msg, _ := out["error"].(string)
	if msg != `header profile "gone" not found` {
		t.Errorf("error = %q, want a plain not-found message", msg)
	}
	if strings.Contains(rec.Body.String(), home) || strings.Contains(rec.Body.String(), "no such file") {
		t.Errorf("response holds a filesystem path: %s", rec.Body)
	}
}

// A profile that exists but can't be read (here: a directory with the profile's name) gives
// a fixed message, never the OS error.
func TestStartCrawlUnreadableHeaderProfileHasNoPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads this on Windows
	path, err := config.HeaderProfilePath("broken")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t)
	rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": "http://127.0.0.1:1/", "header_profile": "broken"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), home) {
		t.Errorf("response holds a filesystem path: %s", rec.Body)
	}
}

func TestSettingsReadErrorDoesNotEchoPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gui.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := New(store.New(t.TempDir()), WithGUISettingsPath(path))
	rec, _ := doJSON(t, srv, http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %s)", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), dir) {
		t.Errorf("response holds a filesystem path: %s", rec.Body)
	}
}

func TestSettingsSaveErrorDoesNotEchoPath(t *testing.T) {
	dir := t.TempDir()
	// A file where the parent directory should be makes MkdirAll fail with a path-bearing error.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	srv := New(store.New(t.TempDir()), WithGUISettingsPath(filepath.Join(blocker, "gui.json")))
	rec, _ := doJSON(t, srv, http.MethodPut, "/api/settings", map[string]any{"theme": "dark"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %s)", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), dir) {
		t.Errorf("response holds a filesystem path: %s", rec.Body)
	}
}

func TestProfileIOErrorDoesNotEchoPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads this on Windows
	// A file where the profiles folder should be makes listing and saving fail with OS errors.
	if err := os.MkdirAll(filepath.Join(home, ".gocrawl"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.HeaderProfileDir(), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t)
	rec, _ := doJSON(t, srv, http.MethodGet, "/api/header-profiles", nil)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), home) {
		t.Errorf("list: status %d, body %s; want a 500 without a path", rec.Code, rec.Body)
	}
	rec, _ = doJSON(t, srv, http.MethodPut, "/api/header-profiles/x", map[string]any{"body": "X-Test: 1"})
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), home) {
		t.Errorf("put: status %d, body %s; want a 500 without a path", rec.Code, rec.Body)
	}
}

// TestProfileErrorHidesRawNotExist covers the error Windows produced in CI: creating the
// profiles folder through a file fails with ERROR_PATH_NOT_FOUND, which Go reports as
// fs.ErrNotExist but which quotes the full path. Only a profile that is genuinely missing may
// pass through as 404; a raw OS error must become the fixed 500.
func TestProfileErrorHidesRawNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret-dir", ".gocrawl", "headers")
	raw := &fs.PathError{Op: "mkdir", Path: path, Err: fs.ErrNotExist}
	rec := httptest.NewRecorder()
	writeProfileError(rec, raw)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret-dir") {
		t.Errorf("raw not-exist: status %d, body %s; want a 500 without a path", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	writeProfileError(rec, &config.HeaderProfileNotFoundError{Name: "uk"})
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `\"uk\"`) {
		t.Errorf("missing profile: status %d, body %s; want a 404 naming the profile", rec.Code, rec.Body)
	}
}
