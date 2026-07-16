package pagination_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/pagination"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

func htmlPage(t *testing.T, html string) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{FinalURL: "https://example.com/", StatusCode: 200, ContentType: "text/html", Doc: doc}
}

func codes(issues []analyze.Issue) map[string]bool {
	out := map[string]bool{}
	for _, is := range issues {
		out[is.Code] = true
	}
	return out
}

func TestPaginationDetected(t *testing.T) {
	p := htmlPage(t, `<html><head>
		<link rel="next" href="https://example.com/page/2">
		<link rel="prev" href="https://example.com/page/1">
	</head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(pagination.New().Analyze(context.Background(), res))
	if !got["pagination-detected"] {
		t.Error("expected pagination-detected")
	}
}

func TestPaginationData(t *testing.T) {
	p := htmlPage(t, `<html><head>
		<link rel="next" href="https://example.com/page/2">
	</head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	issues := pagination.New().Analyze(context.Background(), res)
	var detected analyze.Issue
	found := false
	for _, is := range issues {
		if is.Code == "pagination-detected" {
			detected = is
			found = true
		}
	}
	if !found {
		t.Fatal("expected pagination-detected issue")
	}
	if detected.Data["next"] != "https://example.com/page/2" {
		t.Errorf("expected next href in data, got %v", detected.Data["next"])
	}
	if _, ok := detected.Data["prev"]; ok {
		t.Error("did not expect prev key when no prev link present")
	}
}

func TestPaginationNone(t *testing.T) {
	p := htmlPage(t, `<html><head></head><body><p>no pagination</p></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if len(pagination.New().Analyze(context.Background(), res)) != 0 {
		t.Error("expected no issues on a page without pagination links")
	}
}

// mkPage builds a crawled page at a specific URL/status for cross-reference tests.
func mkPage(t *testing.T, url string, status int, html string) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{RequestedURL: url, FinalURL: url, StatusCode: status, ContentType: "text/html", Doc: doc}
}

func TestPaginationBroken(t *testing.T) {
	p1 := mkPage(t, "https://example.com/page/1", 200,
		`<html><head><link rel="next" href="https://example.com/page/2"></head><body></body></html>`)
	p2 := mkPage(t, "https://example.com/page/2", 404, `<html><head></head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p1, p2}}
	res.Reindex()
	if !codes(pagination.New().Analyze(context.Background(), res))["pagination-broken"] {
		t.Error("expected pagination-broken when the rel=next target returns 404")
	}
}

// TestPaginationResolvesRelativeHref guards against a real bug: a relative rel=next/prev href
// was passed straight to result.Page (an exact-match lookup), so it never matched the crawled
// page's absolute URL and a genuinely broken relative link went unreported.
func TestPaginationResolvesRelativeHref(t *testing.T) {
	p1 := mkPage(t, "https://example.com/page/1", 200,
		`<html><head><link rel="next" href="/page/2"></head><body></body></html>`)
	p2 := mkPage(t, "https://example.com/page/2", 404, `<html><head></head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p1, p2}}
	res.Reindex()
	if !codes(pagination.New().Analyze(context.Background(), res))["pagination-broken"] {
		t.Error("expected pagination-broken for a relative href resolving to a 404 target")
	}
}

// TestPaginationTrailingSlashRedirectNotBroken guards against a false positive: a rel=next
// target reached via its canonical trailing-slash form is not a genuinely broken link.
func TestPaginationTrailingSlashRedirectNotBroken(t *testing.T) {
	p1 := mkPage(t, "https://example.com/page/1", 200,
		`<html><head><link rel="next" href="https://example.com/page/2/"></head><body></body></html>`)
	p2 := mkPage(t, "https://example.com/page/2/", 200, `<html><head></head><body></body></html>`)
	p2.RequestedURL = "https://example.com/page/2"
	p2.Redirects = []crawler.Redirect{{From: "https://example.com/page/2", To: "https://example.com/page/2/", Status: 301}}
	res := &crawler.Result{Pages: []*crawler.Page{p1, p2}}
	res.Reindex()
	if codes(pagination.New().Analyze(context.Background(), res))["pagination-broken"] {
		t.Error("a trailing-slash-only redirect should not be flagged pagination-broken")
	}
}
