package duplicates_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/duplicates"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

func htmlPage(t *testing.T, url, html string) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{FinalURL: url, StatusCode: 200, ContentType: "text/html", Doc: doc}
}

func codes(issues []analyze.Issue) map[string]bool {
	out := map[string]bool{}
	for _, is := range issues {
		out[is.Code] = true
	}
	return out
}

func TestDuplicatesFlagsIdenticalPages(t *testing.T) {
	html := `<html><head><title>Same Title</title></head><body><p>identical body content here</p></body></html>`
	res := &crawler.Result{Pages: []*crawler.Page{
		htmlPage(t, "https://example.com/a", html),
		htmlPage(t, "https://example.com/b", html),
	}}
	got := codes(duplicates.New().Analyze(context.Background(), res))

	for _, want := range []string{"duplicate-content", "duplicate-title"} {
		if !got[want] {
			t.Errorf("expected issue %q, not found", want)
		}
	}
}

// TestDuplicatesIgnoresQueryAndFragmentVariants guards the case from real WordPress crawls:
// the same page reached with different query parameters (?solution=…) or a #fragment is one
// page, not several, so it must not be flagged as duplicating its own title / description.
func TestDuplicatesIgnoresQueryAndFragmentVariants(t *testing.T) {
	html := `<html><head><title>Customer Cases</title>` +
		`<meta name="description" content="Our customer cases"></head>` +
		`<body><p>identical body content here</p></body></html>`
	res := &crawler.Result{Pages: []*crawler.Page{
		htmlPage(t, "https://example.com/customer-case/", html),
		htmlPage(t, "https://example.com/customer-case/?solution=onboarding", html),
		htmlPage(t, "https://example.com/customer-case/?solution=hr-analytics", html),
		htmlPage(t, "https://example.com/customer-case/#top", html),
	}}
	got := codes(duplicates.New().Analyze(context.Background(), res))

	for _, unwanted := range []string{"duplicate-content", "duplicate-title", "duplicate-meta-description"} {
		if got[unwanted] {
			t.Errorf("query/fragment variants of one page should not be flagged as %q", unwanted)
		}
	}
}

// TestDuplicatesFlagsDistinctPathsDespiteVariants ensures collapsing variants does not mask a
// genuine duplicate between two different pages that also carry query strings.
func TestDuplicatesFlagsDistinctPathsDespiteVariants(t *testing.T) {
	html := `<html><head><title>Shared Title</title></head><body><p>shared body text</p></body></html>`
	res := &crawler.Result{Pages: []*crawler.Page{
		htmlPage(t, "https://example.com/page-a/?utm=x", html),
		htmlPage(t, "https://example.com/page-b/?utm=y", html),
	}}
	got := codes(duplicates.New().Analyze(context.Background(), res))
	if !got["duplicate-title"] {
		t.Error("expected duplicate-title across two genuinely distinct paths")
	}
}

func TestDuplicatesDistinctPages(t *testing.T) {
	res := &crawler.Result{Pages: []*crawler.Page{
		htmlPage(t, "https://example.com/a", `<html><head><title>First Title</title></head><body><p>unique first body</p></body></html>`),
		htmlPage(t, "https://example.com/b", `<html><head><title>Second Title</title></head><body><p>different second body</p></body></html>`),
	}}
	got := codes(duplicates.New().Analyze(context.Background(), res))

	for _, unwanted := range []string{"duplicate-content", "duplicate-title", "duplicate-meta-description"} {
		if got[unwanted] {
			t.Errorf("did not expect issue %q on distinct pages", unwanted)
		}
	}
}
