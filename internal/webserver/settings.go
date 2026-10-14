package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/atomicfile"
	"github.com/Patience-dot-devl/gocrawl/internal/crawlrequest"
)

// GUISettings is what the web UI keeps between launches, in gui.json. The UI is served from a
// different port on every desktop launch, so nothing may live in localStorage. It never holds
// a secret: see stripSecrets.
type GUISettings struct {
	Theme         string              `json:"theme"` // "dark" (default) | "light" | "system"
	CrawlDefaults crawlrequest.Params `json:"crawl_defaults"`
}

// settingsView is the GET/PUT /api/settings response: the stored settings plus read-only facts
// about this server. PUT ignores StoreDir.
type settingsView struct {
	GUISettings
	StoreDir string `json:"store_dir"`
}

const (
	defaultTheme    = "dark"
	maxSettingsBody = 1 << 20
)

// Fixed 500 messages: the underlying errors quote the settings file's path.
var (
	errSettingsUnreadable = errors.New("could not read the settings file: check that it is readable, or save the settings again to replace it")
	errSettingsNotSaved   = errors.New("could not save the settings: check that the settings folder is writable and try again")
)

var validThemes = map[string]bool{"dark": true, "light": true, "system": true}

// WithGUISettingsPath sets the file GET and PUT /api/settings read and write (gocrawl serve
// passes ~/.gocrawl/gui.json, see DefaultGUISettingsPath). An empty path, the default, means
// settings are not stored: GET returns the defaults and PUT answers 503. Declared by wave 0.
func WithGUISettingsPath(path string) Option {
	return func(s *Server) { s.guiSettingsPath = path }
}

// DefaultGUISettingsPath is ~/.gocrawl/gui.json, next to the crawl store and header profiles.
func DefaultGUISettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".gocrawl", "gui.json")
	}
	return filepath.Join(home, ".gocrawl", "gui.json")
}

func (s *Server) settingsRoutes() {
	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	settings, err := loadGUISettings(s.guiSettingsPath)
	if err != nil {
		// The error names the settings file's path, which stays out of the response.
		writeError(w, http.StatusInternalServerError, errSettingsUnreadable)
		return
	}
	writeJSON(w, http.StatusOK, s.settingsView(settings))
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	if s.guiSettingsPath == "" {
		writeError(w, http.StatusServiceUnavailable, errors.New("this server was started without a settings file, so settings can't be saved"))
		return
	}
	var in GUISettings
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSettingsBody)).Decode(&in); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("settings are larger than %d bytes", maxSettingsBody))
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	if in.Theme == "" {
		in.Theme = defaultTheme
	}
	if !validThemes[in.Theme] {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown theme %q (want dark, light or system)", in.Theme))
		return
	}
	in.CrawlDefaults = stripSecrets(in.CrawlDefaults)
	if err := validateCrawlDefaults(in.CrawlDefaults); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("crawl_defaults: %w", err))
		return
	}
	if err := saveGUISettings(s.guiSettingsPath, in); err != nil {
		writeError(w, http.StatusInternalServerError, errSettingsNotSaved)
		return
	}
	writeJSON(w, http.StatusOK, s.settingsView(in))
}

func (s *Server) settingsView(g GUISettings) settingsView {
	return settingsView{GUISettings: g, StoreDir: s.store.Root()}
}

func defaultGUISettings() GUISettings {
	return GUISettings{Theme: defaultTheme}
}

// loadGUISettings reads path; a missing file (or no path) gives the defaults. Secrets are
// stripped on load too, so a hand-edited file never feeds one back to the UI.
func loadGUISettings(path string) (GUISettings, error) {
	if path == "" {
		return defaultGUISettings(), nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return defaultGUISettings(), nil
	}
	if err != nil {
		return GUISettings{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var g GUISettings
	if err := json.Unmarshal(data, &g); err != nil {
		return GUISettings{}, fmt.Errorf("reading %s: %w (save the settings again to replace the file)", path, err)
	}
	if !validThemes[g.Theme] {
		g.Theme = defaultTheme
	}
	g.CrawlDefaults = stripSecrets(g.CrawlDefaults)
	return g, nil
}

func saveGUISettings(path string, g GUISettings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicfile.Write(path, 0o600, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(g)
	})
}

// stripSecrets drops every credential a Params can carry: the bearer token, Basic Auth, the
// cookie, inline headers (treated as credentials everywhere else too; header profiles are the
// way to keep headers), and userinfo embedded in the seed or proxy URLs.
func stripSecrets(p crawlrequest.Params) crawlrequest.Params {
	p.BearerToken = ""
	p.BasicAuth = ""
	p.Cookie = ""
	p.Headers = nil
	p.URL = stripUserinfo(p.URL)
	p.Proxy = stripUserinfo(p.Proxy)
	if p.Proxies != nil {
		proxies := make([]string, 0, len(p.Proxies))
		for _, px := range p.Proxies {
			if clean := stripUserinfo(px); clean != "" {
				proxies = append(proxies, clean)
			}
		}
		p.Proxies = proxies
	}
	return p
}

// leadingScheme matches a URL scheme at the very start of a value. A "://" further in, such as
// a redirect parameter in the query, doesn't make the value scheme-qualified.
var leadingScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// stripUserinfo removes user:pass@ from a URL, with or without a scheme. A value that has an
// '@' but doesn't parse is dropped rather than risk storing a credential.
func stripUserinfo(raw string) string {
	if !strings.Contains(raw, "@") {
		return raw
	}
	hasScheme := leadingScheme.MatchString(raw)
	candidate := raw
	if !hasScheme {
		candidate = "http://" + strings.TrimPrefix(raw, "//") // also covers scheme-relative "//u:p@host"
	}
	u, err := url.Parse(candidate)
	if err != nil {
		return ""
	}
	if u.Opaque != "" {
		// Not host-shaped (e.g. "u:p@host" read as scheme "u"): userinfo can't be told apart.
		return ""
	}
	if u.User == nil {
		return raw
	}
	u.User = nil
	out := u.String()
	if !hasScheme {
		out = strings.TrimPrefix(out, "http://")
	}
	return out
}

// validateCrawlDefaults runs the same mapping a crawl start would, so a default the form can't
// use (an unparseable timeout, say) is refused when saved, not when the next crawl starts.
func validateCrawlDefaults(p crawlrequest.Params) error {
	if strings.TrimSpace(p.URL) == "" {
		p.URL = "https://example.invalid"
	}
	_, _, err := p.ToConfig()
	return err
}
