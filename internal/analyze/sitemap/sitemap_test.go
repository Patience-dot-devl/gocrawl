package sitemap_test

import (
	"bytes"
	"compress/gzip"
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

func TestFetchDirect(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(validSitemap)},
	}}
	urls, parsed, invalid, _ := sitemap.Fetch(context.Background(), ff, map[string]bool{"https://example.com/sitemap.xml": false})
	if parsed != 1 {
		t.Fatalf("parsed = %d, want 1", parsed)
	}
	if len(invalid) != 0 {
		t.Fatalf("invalidDeclared = %v, want empty", invalid)
	}
	want := []string{"https://example.com/a", "https://example.com/b"}
	if len(urls) != len(want) {
		t.Fatalf("got %d urls, want %d: %v", len(urls), len(want), urls)
	}
	for _, u := range want {
		if !urls[u] {
			t.Errorf("missing url %q in result %v", u, urls)
		}
	}
}

func TestFetchDirectFlagsDeclaredInvalid(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/broken.xml": {StatusCode: 200, ContentType: "text/html", Body: []byte(softHTML)},
	}}
	_, parsed, invalid, _ := sitemap.Fetch(context.Background(), ff, map[string]bool{"https://example.com/broken.xml": true})
	if parsed != 0 {
		t.Fatalf("parsed = %d, want 0", parsed)
	}
	if len(invalid) != 1 || invalid[0] != "https://example.com/broken.xml" {
		t.Fatalf("invalidDeclared = %v, want [https://example.com/broken.xml]", invalid)
	}
}

// TestFetchDecompressesGzipSitemap guards against a real gap: a sitemap.xml.gz was parsed as
// raw (compressed) bytes and always failed, regardless of body size.
func TestFetchDecompressesGzipSitemap(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write([]byte(validSitemap)); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml.gz": {StatusCode: 200, ContentType: "application/gzip", Body: buf.Bytes()},
	}}
	urls, parsed, invalid, truncated := sitemap.Fetch(context.Background(), ff, map[string]bool{"https://example.com/sitemap.xml.gz": true})
	if parsed != 1 {
		t.Fatalf("parsed = %d, want 1", parsed)
	}
	if len(invalid) != 0 || len(truncated) != 0 {
		t.Fatalf("invalidDeclared = %v, truncatedDeclared = %v, want both empty", invalid, truncated)
	}
	if !urls["https://example.com/a"] || !urls["https://example.com/b"] {
		t.Errorf("gzip sitemap URLs not decoded: %v", urls)
	}
}

// TestFetchFlagsTruncatedSitemapInsteadOfInvalid guards against the false "sitemap-invalid"
// the review flagged: a declared sitemap cut off by the fetcher's body cap is a distinct,
// actionable condition, not a malformed sitemap.
func TestFetchFlagsTruncatedSitemapInsteadOfInvalid(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(validSitemap), Truncated: true},
	}}
	_, parsed, invalid, truncated := sitemap.Fetch(context.Background(), ff, map[string]bool{"https://example.com/sitemap.xml": true})
	if parsed != 0 {
		t.Fatalf("parsed = %d, want 0", parsed)
	}
	if len(invalid) != 0 {
		t.Errorf("invalidDeclared = %v, want empty (should be reported as truncated, not invalid)", invalid)
	}
	if len(truncated) != 1 || truncated[0] != "https://example.com/sitemap.xml" {
		t.Fatalf("truncatedDeclared = %v, want [https://example.com/sitemap.xml]", truncated)
	}
}

// TestAnalyzeEmitsSitemapTruncatedNotMissing checks the Analyze-level wiring: a truncated
// declared sitemap should surface as sitemap-truncated, and sitemap-missing should not also
// fire (a sitemap does exist — it just couldn't be read).
func TestAnalyzeEmitsSitemapTruncatedNotMissing(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(validSitemap), Truncated: true},
	}}
	// Declared via robots.txt so the truncated response is treated as a real, known sitemap
	// rather than an undeclared guessed path (which the analyzer stays silent about to avoid
	// soft-404 noise).
	res := &crawler.Result{
		Seed: "https://example.com/",
		Robots: map[string]*crawler.RobotsData{
			"example.com": {Sitemaps: []string{"https://example.com/sitemap.xml"}},
		},
	}
	issues := sitemap.New(ff).Analyze(context.Background(), res)

	if _, ok := find(issues, "sitemap-truncated"); !ok {
		t.Error("expected sitemap-truncated")
	}
	if _, ok := find(issues, "sitemap-missing"); ok {
		t.Error("sitemap-missing should not fire alongside sitemap-truncated")
	}
	if _, ok := find(issues, "sitemap-invalid"); ok {
		t.Error("sitemap-invalid should not fire for a truncated (not malformed) sitemap")
	}
}
