// Package webserver exposes gocrawl over HTTP: an API to start and inspect crawls, backed by
// the same runner.Run seam the CLI and MCP server use, plus the embedded single-page app
// (see assets.go) that drives it from a browser.
//
// Each feature registers its routes from its own file (crawls.go, sitemap.go, compare.go,
// import.go, redirects.go, settings.go, profiles.go, login.go), so features can be added
// without editing each other's handlers.
package webserver

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

// Server serves the gocrawl web API and embedded frontend.
type Server struct {
	store           *store.Store
	crawls          *jobManager[crawlJob]
	redirects       *jobManager[redirectJob]
	logins          *loginManager
	guiSettingsPath string
	mux             *http.ServeMux
	allowAnyHost    bool
	maxRunning      int
}

// New builds a Server backed by st for crawl history (persisted via --save). By default it
// only answers requests whose Host header is a loopback address and runs at most
// DefaultMaxRunning crawls at once; see the Option values to change either.
func New(st *store.Store, opts ...Option) *Server {
	s := &Server{
		store:      st,
		crawls:     newJobManager[crawlJob](),
		redirects:  newJobManager[redirectJob](),
		logins:     newLoginManager(),
		maxRunning: DefaultMaxRunning,
	}
	for _, o := range opts {
		o(s)
	}
	if _, real := s.logins.browser.(chromedpLoginBrowser); real {
		// Profiles of login windows a killed server never cleaned up. Skipped under a
		// fake browser so tests never touch the real temp directory.
		go sweepStaleLoginProfiles(os.TempDir(), staleLoginProfileAge, time.Now())
	}
	s.mux = http.NewServeMux()
	s.routes()
	return s
}

// Handler returns the http.Handler serving the API and frontend.
func (s *Server) Handler() http.Handler {
	return s.guard(s.mux)
}

// routes registers every feature's API routes, then the frontend as the catch-all.
func (s *Server) routes() {
	s.crawlRoutes()
	s.sitemapRoutes()
	s.compareRoutes()
	s.importRoutes()
	s.redirectRoutes()
	s.settingsRoutes()
	s.profileRoutes()
	s.loginRoutes()
	s.mux.Handle("/", frontendHandler())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
