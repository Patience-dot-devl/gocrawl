package webserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawlrequest"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

func newSettingsServer(t *testing.T) (*Server, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gui.json")
	return New(store.New(t.TempDir()), WithGUISettingsPath(path)), path
}

// settingsRaw sends a request with full control over Content-Type and Origin, for the guard
// and content-type checks that doJSON can't express.
func settingsRaw(t *testing.T, srv *Server, method, path, contentType, origin, body string) *httptest.ResponseRecorder {
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

func TestGetSettingsDefaultsWhenFileMissing(t *testing.T) {
	srv, _ := newSettingsServer(t)
	rec, out := doJSON(t, srv, http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", rec.Code, rec.Body)
	}
	if out["theme"] != "dark" {
		t.Errorf("theme = %v, want dark", out["theme"])
	}
	if _, ok := out["crawl_defaults"].(map[string]any); !ok {
		t.Errorf("crawl_defaults = %v, want an object", out["crawl_defaults"])
	}
	if out["store_dir"] != srv.store.Root() {
		t.Errorf("store_dir = %v, want %q", out["store_dir"], srv.store.Root())
	}
}

func TestPutSettingsPersistsAndStripsSecrets(t *testing.T) {
	srv, path := newSettingsServer(t)
	secrets := []string{"tok-secret", "basic-secret", "cookie-secret", "header-secret", "proxy-secret", "url-secret"}
	body := map[string]any{
		"theme": "light",
		"crawl_defaults": map[string]any{
			"url":            "https://alice:url-secret@example.com/",
			"max_pages":      50,
			"timeout":        "9s",
			"header_profile": "uk",
			"bearer_token":   "tok-secret",
			"basic_auth":     "alice:basic-secret",
			"cookie":         "sid=cookie-secret",
			"headers":        []string{"X-Key: header-secret"},
			"proxy":          "http://pu:proxy-secret@proxy.example:8080",
			"proxies":        []string{"socks5://pu:proxy-secret@proxy2.example:1080"},
		},
	}
	rec, out := doJSON(t, srv, http.MethodPut, "/api/settings", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", rec.Code, rec.Body)
	}
	if out["theme"] != "light" {
		t.Errorf("theme = %v, want light", out["theme"])
	}
	defaults, _ := out["crawl_defaults"].(map[string]any)
	if defaults["max_pages"] != float64(50) || defaults["timeout"] != "9s" || defaults["header_profile"] != "uk" {
		t.Errorf("crawl_defaults lost non-secret values: %v", defaults)
	}
	if defaults["url"] != "https://example.com/" {
		t.Errorf("url = %v, want the userinfo stripped", defaults["url"])
	}
	if defaults["proxy"] != "http://proxy.example:8080" {
		t.Errorf("proxy = %v, want the userinfo stripped", defaults["proxy"])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range secrets {
		if strings.Contains(string(data), s) {
			t.Errorf("gui.json holds %q:\n%s", s, data)
		}
		if strings.Contains(rec.Body.String(), s) {
			t.Errorf("PUT response echoes %q", s)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("gui.json mode = %o, want 600", perm)
		}
	}

	// A new server reading the same file (the next launch) sees the saved settings.
	again := New(store.New(t.TempDir()), WithGUISettingsPath(path))
	_, reloaded := doJSON(t, again, http.MethodGet, "/api/settings", nil)
	if reloaded["theme"] != "light" {
		t.Errorf("reloaded theme = %v, want light", reloaded["theme"])
	}
	if d, _ := reloaded["crawl_defaults"].(map[string]any); d["max_pages"] != float64(50) {
		t.Errorf("reloaded crawl_defaults = %v, want max_pages 50", d)
	}
}

func TestPutSettingsValidates(t *testing.T) {
	srv, path := newSettingsServer(t)
	cases := map[string]map[string]any{
		"unknown theme":    {"theme": "neon", "crawl_defaults": map[string]any{}},
		"bad timeout":      {"theme": "dark", "crawl_defaults": map[string]any{"timeout": "soon"}},
		"bad max_duration": {"theme": "dark", "crawl_defaults": map[string]any{"max_duration": "later"}},
	}
	for name, body := range cases {
		if rec, _ := doJSON(t, srv, http.MethodPut, "/api/settings", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("gui.json was written despite invalid input (stat err %v)", err)
	}
}

func TestPutSettingsEmptyThemeMeansDark(t *testing.T) {
	srv, _ := newSettingsServer(t)
	rec, out := doJSON(t, srv, http.MethodPut, "/api/settings", map[string]any{"crawl_defaults": map[string]any{}})
	if rec.Code != http.StatusOK || out["theme"] != "dark" {
		t.Errorf("status %d, theme %v; want 200 and dark", rec.Code, out["theme"])
	}
}

func TestPutSettingsGuards(t *testing.T) {
	srv, path := newSettingsServer(t)
	if rec := settingsRaw(t, srv, http.MethodPut, "/api/settings", "text/plain", "", `{"theme":"light"}`); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: status = %d, want 415", rec.Code)
	}
	if rec := settingsRaw(t, srv, http.MethodPut, "/api/settings", "application/json", "http://evil.example", `{"theme":"light"}`); rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin: status = %d, want 403", rec.Code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("gui.json was written by a rejected request")
	}
}

// A hand-edited gui.json never feeds a secret back to the UI.
func TestGetSettingsStripsHandEditedSecrets(t *testing.T) {
	srv, path := newSettingsServer(t)
	if err := os.WriteFile(path, []byte(`{"theme":"system","crawl_defaults":{"bearer_token":"tok-secret","cookie":"sid=1","max_pages":7}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	rec, out := doJSON(t, srv, http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusOK || out["theme"] != "system" {
		t.Fatalf("status %d, theme %v", rec.Code, out["theme"])
	}
	if strings.Contains(rec.Body.String(), "tok-secret") || strings.Contains(rec.Body.String(), "sid=1") {
		t.Errorf("GET returned a secret: %s", rec.Body)
	}
	if d, _ := out["crawl_defaults"].(map[string]any); d["max_pages"] != float64(7) {
		t.Errorf("crawl_defaults = %v, want max_pages 7", d)
	}
}

func TestGetSettingsReportsCorruptFile(t *testing.T) {
	srv, path := newSettingsServer(t)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec, out := doJSON(t, srv, http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	// The binding rule: no filesystem path in a response. The message says what to do instead.
	if msg, _ := out["error"].(string); strings.Contains(msg, path) || !strings.Contains(msg, "settings file") {
		t.Errorf("error %q should describe the settings file without its path", msg)
	}
}

func TestPutSettingsWithoutPathIsUnavailable(t *testing.T) {
	srv := newTestServer(t)
	if rec, _ := doJSON(t, srv, http.MethodPut, "/api/settings", map[string]any{"theme": "light"}); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

// Every string-carrying Params field must be classified here. A new field that could hold a
// credential fails this test until someone decides, so stripSecrets can't silently miss it.
func TestStripSecretsClassifiesEveryTextField(t *testing.T) {
	secret := map[string]bool{
		"URL": true, "Proxy": true, "Proxies": true, // userinfo is stripped, the rest is kept
		"BasicAuth": true, "BearerToken": true, "Cookie": true, "Headers": true,
	}
	plain := map[string]bool{
		"MaxDuration": true, "Render": true, "Analyzers": true, "Include": true, "Exclude": true,
		"Timeout": true, "UserAgent": true, "UserAgents": true, "UserAgentRotation": true,
		"ProxyRotation": true, "HeaderProfile": true, // a profile name, not its values
	}
	typ := reflect.TypeOf(crawlrequest.Params{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		isText := f.Type.Kind() == reflect.String || (f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.String)
		if !isText {
			continue
		}
		if secret[f.Name] == plain[f.Name] {
			t.Errorf("Params.%s is a text field that is not classified exactly once as secret or plain: decide whether stripSecrets must clear it, then list it in this test", f.Name)
		}
	}

	// Each field listed as secret must actually be cleaned (userinfo fields) or cleared.
	full := crawlrequest.Params{
		URL: "https://u:p@example.com/", Proxy: "http://u:p@px:1", Proxies: []string{"http://u:p@px2:1"},
		BasicAuth: "u:p", BearerToken: "t", Cookie: "c=1", Headers: []string{"K: v"},
	}
	out, err := json.Marshal(stripSecrets(full))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"u:p", "\"t\"", "c=1", "K: v"} {
		if strings.Contains(string(out), s) {
			t.Errorf("stripSecrets left %q in %s", s, out)
		}
	}
}

func TestStripUserinfoNeverKeepsAPassword(t *testing.T) {
	for _, raw := range []string{
		"u:p@host/path?r=http://x",
		"u:p@host",
		"u:p@host:8080/a",
		"https://u:p@host/x",
		"//u:p@host/x",
	} {
		got := stripUserinfo(raw)
		if strings.Contains(got, "u:p") || strings.Contains(got, ":p@") {
			t.Errorf("stripUserinfo(%q) = %q, kept the password", raw, got)
		}
	}
	if got := stripUserinfo("https://example.com/@x"); got != "https://example.com/@x" {
		t.Errorf("an @ in the path changed the URL: %q", got)
	}
}

func TestPutSettingsDropsSchemelessUserinfoFromFileAndResponse(t *testing.T) {
	srv, path := newSettingsServer(t)
	body := map[string]any{"theme": "dark", "crawl_defaults": map[string]any{
		"url":     "u:p@host/path?r=http://x",
		"proxy":   "u:p@host",
		"proxies": []string{"u:p@host/path?r=http://x"},
	}}
	rec, _ := doJSON(t, srv, http.MethodPut, "/api/settings", body)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("status %d: %v", rec.Code, err)
	}
	for name, s := range map[string]string{"response": rec.Body.String(), "gui.json": string(data)} {
		if strings.Contains(s, "u:p") || strings.Contains(s, ":p@") {
			t.Errorf("%s holds the password: %s", name, s)
		}
	}
}

func TestPutSettingsTooLargeIs413(t *testing.T) {
	srv, path := newSettingsServer(t)
	big := `{"theme":"dark","crawl_defaults":{"url":"` + strings.Repeat("a", maxSettingsBody) + `"}}`
	if rec := settingsRaw(t, srv, http.MethodPut, "/api/settings", "application/json", "", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("gui.json was written by an oversized request")
	}
}
