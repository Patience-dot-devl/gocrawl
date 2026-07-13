package redirectcheck_test

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func TestRunOrdersResultsAndSkipsOutOfScope(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(testSitemap)},
		"https://example.com/old":         {StatusCode: 200, FinalURL: "https://example.com/a", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/a", Status: 301}}},
		"https://example.com/a":           {StatusCode: 200, FinalURL: "https://example.com/a"},
	}}
	rules := []redirectcheck.Rule{
		{Original: "/old", Target: "/a", IgnoreTrailingSlash: "TRUE", IgnoreProtocol: "TRUE", DisableIfPageExists: "TRUE"},
		{Original: "/gone", Target: "https://other-site.com/somewhere", DisableIfPageExists: "TRUE"},
	}
	results, err := redirectcheck.Run(context.Background(), rules, redirectcheck.RunOptions{
		Domain: "example.com", Fetcher: ff, Concurrency: 2,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Verdict != redirectcheck.VerdictOK {
		t.Errorf("row 0 verdict = %q, notes = %v, want ok", results[0].Verdict, results[0].Notes)
	}
	if results[1].Verdict != redirectcheck.VerdictSkippedExternal {
		t.Errorf("row 1 verdict = %q, want skipped-external", results[1].Verdict)
	}
}

func TestRunErrorsWhenSitemapUnreachable(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{}}
	rules := []redirectcheck.Rule{{Original: "/old", Target: "/new"}}
	if _, err := redirectcheck.Run(context.Background(), rules, redirectcheck.RunOptions{Domain: "example.com", Fetcher: ff}); err == nil {
		t.Fatal("expected an error when the sitemap can't be reached")
	}
}
