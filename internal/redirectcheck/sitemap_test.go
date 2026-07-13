package redirectcheck_test

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

// fakeFetcher serves canned responses keyed by URL; unknown URLs return a 404. Shared by every
// test file in this package.
type fakeFetcher struct{ pages map[string]*crawler.Page }

func (f fakeFetcher) Fetch(_ context.Context, rawURL string) (*crawler.Page, error) {
	if p, ok := f.pages[rawURL]; ok {
		return p, nil
	}
	return &crawler.Page{RequestedURL: rawURL, StatusCode: 404}, nil
}

const testSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/a</loc></url>
  <url><loc>https://example.com/b</loc></url>
</urlset>`

func TestDiscoverSitemapDefault(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(testSitemap)},
	}}
	urls, err := redirectcheck.DiscoverSitemap(context.Background(), ff, "example.com", "")
	if err != nil {
		t.Fatalf("DiscoverSitemap: %v", err)
	}
	if !urls["example.com/a"] || !urls["example.com/b"] {
		t.Errorf("got %v, missing expected normalized URLs", urls)
	}
}

func TestDiscoverSitemapOverride(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/custom-sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(testSitemap)},
	}}
	urls, err := redirectcheck.DiscoverSitemap(context.Background(), ff, "example.com", "https://example.com/custom-sitemap.xml")
	if err != nil {
		t.Fatalf("DiscoverSitemap: %v", err)
	}
	if len(urls) != 2 {
		t.Errorf("got %d urls, want 2: %v", len(urls), urls)
	}
}

func TestDiscoverSitemapNotFoundErrors(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{}}
	if _, err := redirectcheck.DiscoverSitemap(context.Background(), ff, "example.com", ""); err == nil {
		t.Fatal("expected an error when no sitemap is reachable")
	}
}
