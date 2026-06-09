package landing_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/landing"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

func doc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	d, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return d
}

// landingPage builds a single-page Result whose own FinalURL carries UTM params.
func landingPage(t *testing.T, finalURL, html string) *crawler.Result {
	return &crawler.Result{Pages: []*crawler.Page{{FinalURL: finalURL, StatusCode: 200, ContentType: "text/html", Doc: doc(t, html)}}}
}

func run(res *crawler.Result) []analyze.Issue {
	return landing.New().Analyze(context.Background(), res)
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func TestKeywordAligned(t *testing.T) {
	res := landingPage(t,
		"https://example.com/lp?utm_medium=cpc&utm_campaign=blue+running+shoes",
		`<html><head><title>Blue Running Shoes Sale</title></head><body><h1>Running Shoes</h1></body></html>`)
	if _, ok := find(run(res), "landing-keyword-aligned"); !ok {
		t.Error("expected landing-keyword-aligned")
	}
}

func TestKeywordMismatch(t *testing.T) {
	res := landingPage(t,
		"https://example.com/lp?utm_medium=cpc&utm_campaign=blue+running+shoes",
		`<html><head><title>Welcome</title></head><body><h1>Home</h1></body></html>`)
	is, ok := find(run(res), "landing-keyword-mismatch")
	if !ok {
		t.Fatal("expected landing-keyword-mismatch")
	}
	if is.Severity != analyze.Warning {
		t.Errorf("severity = %v, want warning", is.Severity)
	}
}

func TestNoindex(t *testing.T) {
	res := landingPage(t,
		"https://example.com/lp?utm_campaign=shoes",
		`<html><head><title>Shoes</title><meta name="robots" content="noindex,follow"></head><body><h1>Shoes</h1></body></html>`)
	is, ok := find(run(res), "landing-noindex")
	if !ok {
		t.Fatal("expected landing-noindex")
	}
	if is.Severity != analyze.Error {
		t.Errorf("severity = %v, want error", is.Severity)
	}
}

func TestNotHTTPS(t *testing.T) {
	res := landingPage(t,
		"http://example.com/lp?utm_campaign=shoes",
		`<html><head><title>Shoes</title></head><body><h1>Shoes</h1></body></html>`)
	if _, ok := find(run(res), "landing-not-https"); !ok {
		t.Error("expected landing-not-https")
	}
}

func TestNotALandingPage(t *testing.T) {
	res := landingPage(t,
		"https://example.com/about",
		`<html><head><title>About</title></head><body><h1>About</h1></body></html>`)
	if len(run(res)) != 0 {
		t.Errorf("expected no issues for a non-landing page, got %d", len(run(res)))
	}
}

// TestCrossPageDetection exercises the link-based path: page A links to page B with UTM
// campaign params, so B is a landing page even though B's own URL has no UTM.
func TestCrossPageDetection(t *testing.T) {
	pageA := &crawler.Page{
		FinalURL:   "https://example.com/",
		StatusCode: 200, ContentType: "text/html",
		Doc:   doc(t, `<html><head><title>Home</title></head><body></body></html>`),
		Links: []crawler.Link{{URL: "https://example.com/lp?utm_campaign=running+shoes", External: false}},
	}
	pageB := &crawler.Page{
		FinalURL:   "https://example.com/lp",
		StatusCode: 200, ContentType: "text/html",
		Doc: doc(t, `<html><head><title>Running Shoes Store</title></head><body><h1>Running Shoes</h1></body></html>`),
	}
	res := &crawler.Result{Pages: []*crawler.Page{pageA, pageB}}
	issues := run(res)
	is, ok := find(issues, "landing-keyword-aligned")
	if !ok {
		t.Fatal("expected B to be detected as an aligned landing page via the inbound UTM link")
	}
	if is.URL != "https://example.com/lp" {
		t.Errorf("issue URL = %s, want page B", is.URL)
	}
}
