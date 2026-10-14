package webserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sitemapTarget(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", base)
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>%[1]s/</loc></url><url><loc>%[1]s/a</loc></url><url><loc>%[1]s/blog/x</loc></url></urlset>`, base)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestSitemapPreview(t *testing.T) {
	srv := newTestServer(t)
	target := sitemapTarget(t)

	rec, out := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{"url": target.URL})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %v", rec.Code, out)
	}
	if out["found"] != true || out["total"] != float64(3) || out["via_robots"] != true {
		t.Errorf("preview = %v, want found, 3, via robots.txt", out)
	}
	if sources, _ := out["sources"].([]any); len(sources) != 1 {
		t.Errorf("sources = %v, want one", out["sources"])
	}

	_, scoped := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{"url": target.URL, "exclude": []string{"/blog/"}})
	if scoped["total"] != float64(2) {
		t.Errorf("total with /blog/ excluded = %v, want 2", scoped["total"])
	}
}

func TestSitemapPreviewNoSitemap(t *testing.T) {
	srv := newTestServer(t)
	target := startCrawlTarget(t) // HTML for every path: no parseable sitemap
	rec, out := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{"url": target.URL})
	if rec.Code != http.StatusOK || out["found"] != false || out["total"] != float64(0) {
		t.Errorf("status %d, preview %v, want 200 not found", rec.Code, out)
	}
}

func TestSitemapPreviewRejectsBadInput(t *testing.T) {
	srv := newTestServer(t)
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{}); rec.Code != http.StatusBadRequest {
		t.Errorf("missing url: status %d, want 400", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{"url": "https://example.com", "exclude": []string{"("}}); rec.Code != http.StatusBadRequest {
		t.Errorf("bad regex: status %d, want 400", rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sitemap-preview", strings.NewReader(`{"url":"https://example.com"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: status %d, want 415", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/sitemap-preview", strings.NewReader(`{"url":"https://example.com"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example")
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin: status %d, want 403", rec.Code)
	}
}

// A URL with embedded credentials that fails to parse must not have them echoed back.
func TestSitemapPreviewErrorsDoNotEchoUserinfo(t *testing.T) {
	srv := newTestServer(t)
	for _, u := range []string{"https://user:s3cret@exa mple.com/", "https://user:s3cret@host:badport/"} {
		rec, _ := doJSON(t, srv, http.MethodPost, "/api/sitemap-preview", map[string]any{"url": u})
		if strings.Contains(rec.Body.String(), "s3cret") {
			t.Errorf("url %q: response leaks the password: %s", u, rec.Body.String())
		}
	}
}
