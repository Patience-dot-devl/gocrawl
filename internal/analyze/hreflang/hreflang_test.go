package hreflang_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/hreflang"
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

func TestHreflangInvalidCode(t *testing.T) {
	p := htmlPage(t, `<html><head>
		<link rel="alternate" hreflang="english" href="https://example.com/en">
		<link rel="alternate" hreflang="x-default" href="https://example.com/">
	</head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if !codes(hreflang.New().Analyze(context.Background(), res))["hreflang-invalid-code"] {
		t.Error("expected hreflang-invalid-code for 'english'")
	}
}

func TestHreflangValidCodes(t *testing.T) {
	p := htmlPage(t, `<html><head>
		<link rel="alternate" hreflang="en" href="https://example.com/en">
		<link rel="alternate" hreflang="en-US" href="https://example.com/en-us">
		<link rel="alternate" hreflang="x-default" href="https://example.com/">
	</head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(hreflang.New().Analyze(context.Background(), res))
	if got["hreflang-invalid-code"] {
		t.Error("did not expect hreflang-invalid-code for valid codes")
	}
	if got["hreflang-missing-x-default"] {
		t.Error("did not expect hreflang-missing-x-default when x-default present")
	}
}

// TestHreflangValidCodesRealWorldVariants guards against a real false-positive bug: a
// hand-rolled regex (^[a-z]{2}(-[A-Z]{2})?$) rejected several legitimate real-world hreflang
// values seen in production sites.
func TestHreflangValidCodesRealWorldVariants(t *testing.T) {
	for _, code := range []string{"es-419", "zh-Hant", "zh-Hans", "en-us", "fil"} {
		t.Run(code, func(t *testing.T) {
			p := htmlPage(t, `<html><head>
				<link rel="alternate" hreflang="`+code+`" href="https://example.com/x">
				<link rel="alternate" hreflang="x-default" href="https://example.com/">
			</head><body></body></html>`)
			res := &crawler.Result{Pages: []*crawler.Page{p}}
			if got := codes(hreflang.New().Analyze(context.Background(), res)); got["hreflang-invalid-code"] {
				t.Errorf("hreflang=%q incorrectly flagged as hreflang-invalid-code", code)
			}
		})
	}
}

func TestHreflangMissingXDefault(t *testing.T) {
	p := htmlPage(t, `<html><head>
		<link rel="alternate" hreflang="en" href="https://example.com/en">
		<link rel="alternate" hreflang="fr" href="https://example.com/fr">
	</head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if !codes(hreflang.New().Analyze(context.Background(), res))["hreflang-missing-x-default"] {
		t.Error("expected hreflang-missing-x-default")
	}
}

func TestHreflangNoLinks(t *testing.T) {
	p := htmlPage(t, `<html><head></head><body><p>no hreflang</p></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if len(hreflang.New().Analyze(context.Background(), res)) != 0 {
		t.Error("expected no issues on a page without hreflang links")
	}
}

// mkPage builds a crawled page at a specific URL for cross-reference tests.
func mkPage(t *testing.T, url, html string) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{RequestedURL: url, FinalURL: url, StatusCode: 200, ContentType: "text/html", Doc: doc}
}

func issuesFor(issues []analyze.Issue, url, code string) bool {
	for _, is := range issues {
		if is.URL == url && is.Code == code {
			return true
		}
	}
	return false
}

func TestHreflangNoReturnLink(t *testing.T) {
	// /en references itself and /fr; /fr references only itself — no return link to /en.
	a := mkPage(t, "https://example.com/en", `<html><head>
		<link rel="alternate" hreflang="en" href="https://example.com/en">
		<link rel="alternate" hreflang="fr" href="https://example.com/fr">
	</head><body></body></html>`)
	b := mkPage(t, "https://example.com/fr", `<html><head>
		<link rel="alternate" hreflang="fr" href="https://example.com/fr">
	</head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{a, b}}
	res.Reindex()
	issues := hreflang.New().Analyze(context.Background(), res)
	if !issuesFor(issues, "https://example.com/en", "hreflang-no-return-link") {
		t.Error("expected hreflang-no-return-link on /en (no reciprocal link from /fr)")
	}
}

func TestHreflangMissingSelf(t *testing.T) {
	// Cluster points only at other URLs, never at the page's own URL.
	a := mkPage(t, "https://example.com/en", `<html><head>
		<link rel="alternate" hreflang="fr" href="https://example.com/fr">
		<link rel="alternate" hreflang="x-default" href="https://example.com/intl">
	</head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{a}}
	res.Reindex()
	if !codes(hreflang.New().Analyze(context.Background(), res))["hreflang-missing-self"] {
		t.Error("expected hreflang-missing-self when no cluster href points to the page itself")
	}
}
