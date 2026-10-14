package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawlrequest"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// sitemapPreviewTimeout bounds one preview: robots.txt plus a sitemap index two levels deep.
const sitemapPreviewTimeout = 20 * time.Second

// sitemapPreviewRequest is the POST /api/sitemap-preview body: the scope fields of a crawl.
type sitemapPreviewRequest struct {
	URL             string   `json:"url"`
	Include         []string `json:"include,omitempty"`
	Exclude         []string `json:"exclude,omitempty"`
	AllowSubdomains bool     `json:"allow_subdomains,omitempty"`
}

// sitemapRoutes registers POST /api/sitemap-preview, which runs only the sitemap discovery
// step for the New crawl form.
func (s *Server) sitemapRoutes() {
	s.mux.HandleFunc("POST /api/sitemap-preview", s.handleSitemapPreview)
}

// handleSitemapPreview runs only the crawl's sitemap discovery step, so the New crawl form can
// show how many in-scope pages the sitemap lists before anything is crawled.
func (s *Server) handleSitemapPreview(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var req sitemapPreviewRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	subdomains := req.AllowSubdomains
	cfg, seed, err := crawlrequest.Params{
		URL:        req.URL,
		Include:    req.Include,
		Exclude:    req.Exclude,
		Subdomains: &subdomains,
	}.ToConfig()
	if err != nil {
		writeError(w, http.StatusBadRequest, redactURLError(err))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), sitemapPreviewTimeout)
	defer cancel()
	preview, err := runner.PreviewSitemap(ctx, cfg, seed)
	if err != nil {
		writeError(w, http.StatusBadRequest, redactURLError(err))
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

// redactURLError returns err with the URL a *url.Error quotes reduced to its redacted form, so a
// seed that carries userinfo never comes back in a response body. A URL that does not parse is
// dropped rather than echoed, since its userinfo cannot be told apart from the rest.
func redactURLError(err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	if u, perr := url.Parse(ue.URL); perr == nil {
		return fmt.Errorf("%s %q: %w", ue.Op, u.Redacted(), ue.Err)
	}
	return fmt.Errorf("%s: invalid URL: %w", ue.Op, ue.Err)
}
