package sitemap_test

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/sitemap"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// fakeFetcher serves canned responses keyed by URL; unknown URLs return a 404.
type fakeFetcher struct{ pages map[string]*crawler.Page }

func (f fakeFetcher) Fetch(_ context.Context, rawURL string) (*crawler.Page, error) {
	if p, ok := f.pages[rawURL]; ok {
		return p, nil
	}
	return &crawler.Page{RequestedURL: rawURL, StatusCode: 404}, nil
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

const validSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/a</loc></url>
  <url><loc>https://example.com/b</loc></url>
</urlset>`

const softHTML = `<!DOCTYPE html><html><body>Page not found</body></html>`

// TestGuessedIndexSoft404NotFlagged is the regression for the reported bug: the real sitemap
// lives at /sitemap.xml, while the guessed /sitemap_index.xml returns an HTTP 200 HTML
// soft-404. The analyzer must use the real sitemap and stay silent about the guessed path.
func TestGuessedIndexSoft404NotFlagged(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml":       {StatusCode: 200, ContentType: "application/xml", Body: []byte(validSitemap)},
		"https://example.com/sitemap_index.xml": {StatusCode: 200, ContentType: "text/html", Body: []byte(softHTML)},
	}}
	res := &crawler.Result{Seed: "https://example.com/"}
	issues := sitemap.New(ff).Analyze(context.Background(), res)

	if is, ok := find(issues, "sitemap-invalid"); ok {
		t.Errorf("guessed soft-404 path should not be flagged invalid-sitemap, got URL %q", is.URL)
	}
	if _, ok := find(issues, "sitemap-missing"); ok {
		t.Error("a real /sitemap.xml exists; should not report no-sitemap")
	}
	if _, ok := find(issues, "sitemap-coverage"); !ok {
		t.Error("expected sitemap-coverage from the parsed /sitemap.xml")
	}
}

// TestDeclaredBrokenSitemapFlagged: a sitemap declared in robots.txt that won't parse is a
// genuine misconfiguration and must still be flagged.
func TestDeclaredBrokenSitemapFlagged(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/broken.xml": {StatusCode: 200, ContentType: "text/html", Body: []byte(softHTML)},
	}}
	res := &crawler.Result{
		Seed:   "https://example.com/",
		Robots: map[string]*crawler.RobotsData{"example.com": {Sitemaps: []string{"https://example.com/broken.xml"}}},
	}
	if _, ok := find(sitemap.New(ff).Analyze(context.Background(), res), "sitemap-invalid"); !ok {
		t.Error("a declared sitemap that won't parse should be flagged invalid-sitemap")
	}
}

// TestNoSitemapWhenOnlySoft404s: when every candidate is a soft-404, report no-sitemap.
func TestNoSitemapWhenOnlySoft404s(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml":       {StatusCode: 200, ContentType: "text/html", Body: []byte(softHTML)},
		"https://example.com/sitemap_index.xml": {StatusCode: 200, ContentType: "text/html", Body: []byte(softHTML)},
	}}
	res := &crawler.Result{Seed: "https://example.com/"}
	issues := sitemap.New(ff).Analyze(context.Background(), res)
	if _, ok := find(issues, "sitemap-missing"); !ok {
		t.Error("expected no-sitemap when all candidates are HTML soft-404s")
	}
	if _, ok := find(issues, "sitemap-invalid"); ok {
		t.Error("guessed soft-404 paths should not be flagged invalid-sitemap")
	}
}
