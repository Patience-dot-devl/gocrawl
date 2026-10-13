package webserver

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

func TestHeaderProfileRoutes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := newTestServer(t)

	rec, out := doJSON(t, srv, http.MethodGet, "/api/header-profiles", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status = %d (body %s)", rec.Code, rec.Body)
	}
	if list, ok := out["profiles"].([]any); !ok || len(list) != 0 {
		t.Fatalf("profiles = %#v, want an empty list", out["profiles"])
	}

	if rec, _ := doJSON(t, srv, http.MethodPut, "/api/header-profiles/uk", map[string]string{"body": "Signature: abc\n"}); rec.Code != http.StatusNoContent {
		t.Fatalf("put: status = %d, want 204 (body %s)", rec.Code, rec.Body)
	}
	_, out = doJSON(t, srv, http.MethodGet, "/api/header-profiles", nil)
	if got := out["profiles"]; !reflect.DeepEqual(got, []any{"uk"}) {
		t.Errorf("profiles = %v, want [uk]", got)
	}
	rec, out = doJSON(t, srv, http.MethodGet, "/api/header-profiles/uk", nil)
	if rec.Code != http.StatusOK || out["name"] != "uk" || out["body"] != "Signature: abc\n" {
		t.Errorf("get: status %d, body %v", rec.Code, out)
	}

	rec, out = doJSON(t, srv, http.MethodPut, "/api/header-profiles/uk", map[string]string{"body": "not a header"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid body: status = %d, want 400", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "Name: value") {
		t.Errorf("invalid body: error %q doesn't explain the format", msg)
	}
	if _, out = doJSON(t, srv, http.MethodGet, "/api/header-profiles/uk", nil); out["body"] != "Signature: abc\n" {
		t.Errorf("a rejected save changed the profile: %v", out["body"])
	}

	if rec, _ := doJSON(t, srv, http.MethodDelete, "/api/header-profiles/uk", nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete: status = %d, want 204", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodDelete, "/api/header-profiles/uk", nil); rec.Code != http.StatusNotFound {
		t.Errorf("delete again: status = %d, want 404", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodGet, "/api/header-profiles/uk", nil); rec.Code != http.StatusNotFound {
		t.Errorf("get deleted: status = %d, want 404", rec.Code)
	}
}

func TestHeaderProfileRoutesRejectBadNames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := newTestServer(t)
	for _, name := range []string{".hidden", "bad%20name"} {
		if rec, _ := doJSON(t, srv, http.MethodPut, "/api/header-profiles/"+name, map[string]string{"body": "X-A: 1"}); rec.Code != http.StatusBadRequest {
			t.Errorf("put %q: status = %d, want 400", name, rec.Code)
		}
		if rec, _ := doJSON(t, srv, http.MethodGet, "/api/header-profiles/"+name, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("get %q: status = %d, want 400", name, rec.Code)
		}
	}
	// An encoded slash must never reach the file system as a path separator.
	rec := settingsRaw(t, srv, http.MethodPut, "/api/header-profiles/a%2Fb", "application/json", "", `{"body":"X-A: 1"}`)
	if rec.Code < 400 {
		t.Errorf("put a%%2Fb: status = %d, want an error", rec.Code)
	}
	if names, _ := config.ListHeaderProfiles(); len(names) != 0 {
		t.Errorf("bad names created profiles: %v", names)
	}
}

func TestHeaderProfileRoutesGuards(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := config.SaveHeaderProfile("uk", "X-A: 1\n"); err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t)
	if rec := settingsRaw(t, srv, http.MethodPut, "/api/header-profiles/uk", "text/plain", "", `{"body":"X-B: 2"}`); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain put: status = %d, want 415", rec.Code)
	}
	if rec := settingsRaw(t, srv, http.MethodDelete, "/api/header-profiles/uk", "", "http://evil.example", ""); rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin delete: status = %d, want 403", rec.Code)
	}
	if body, err := config.ReadHeaderProfile("uk"); err != nil || body != "X-A: 1\n" {
		t.Errorf("profile changed by a rejected request: %q, %v", body, err)
	}
}

func TestHeaderProfileRoutesRejectOversizedBody(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := newTestServer(t)
	big := `{"body":"X-A: ` + strings.Repeat("a", maxProfileBody) + `"}`
	rec := settingsRaw(t, srv, http.MethodPut, "/api/header-profiles/uk", "application/json", "", big)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized put: status = %d, want 413", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "aaaa") {
		t.Errorf("413 response echoes the body: %.80s", rec.Body)
	}
	if names, _ := config.ListHeaderProfiles(); len(names) != 0 {
		t.Errorf("oversized put created profiles: %v", names)
	}
}
