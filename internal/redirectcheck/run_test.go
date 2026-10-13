package redirectcheck_test

import (
	"context"
	"sync"
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

func TestRunReportsEveryRowThroughOnRow(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(testSitemap)},
		"https://example.com/old":         {StatusCode: 200, FinalURL: "https://example.com/a", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/a", Status: 301}}},
		"https://example.com/a":           {StatusCode: 200, FinalURL: "https://example.com/a"},
	}}
	inScope := redirectcheck.Rule{Original: "/old", Target: "/a", IgnoreTrailingSlash: "TRUE", IgnoreProtocol: "TRUE"}
	rules := []redirectcheck.Rule{
		inScope, inScope, inScope, inScope,
		{Original: "/gone", Target: "https://other-site.com/x"},  // external: skipped
		{Original: "/blog/(?P<slug>.*)", Target: "/news/{slug}"}, // dynamic: skipped
	}

	var mu sync.Mutex
	var seen []int
	totals := map[int]bool{}
	_, err := redirectcheck.Run(context.Background(), rules, redirectcheck.RunOptions{
		Domain: "example.com", Fetcher: ff, Concurrency: 3,
		OnRow: func(done, total int) {
			mu.Lock()
			defer mu.Unlock()
			seen = append(seen, done)
			totals[total] = true
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != len(rules) {
		t.Fatalf("OnRow called %d times, want %d", len(seen), len(rules))
	}
	for i, d := range seen {
		if d != i+1 {
			t.Fatalf("OnRow done sequence = %v, want 1..%d in order", seen, len(rules))
		}
	}
	if len(totals) != 1 || !totals[len(rules)] {
		t.Fatalf("OnRow totals = %v, want only %d", totals, len(rules))
	}
}

func TestRunWithoutOnRowStillWorks(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(testSitemap)},
	}}
	rules := []redirectcheck.Rule{{Original: "/gone", Target: "https://other-site.com/x"}}
	if _, err := redirectcheck.Run(context.Background(), rules, redirectcheck.RunOptions{Domain: "example.com", Fetcher: ff}); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
