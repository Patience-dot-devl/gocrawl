package seo_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/seo"
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

func TestSEOFlagsMissingElements(t *testing.T) {
	p := htmlPage(t, `<html><head></head><body><p>no metadata here at all</p></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(seo.New().Analyze(context.Background(), res))

	for _, want := range []string{"missing-title", "missing-meta-description", "missing-h1", "missing-canonical", "missing-viewport", "missing-opengraph"} {
		if !got[want] {
			t.Errorf("expected issue %q, not found", want)
		}
	}
}

func TestSEOCleanPage(t *testing.T) {
	p := htmlPage(t, `<html lang="en"><head>
		<meta charset="utf-8">
		<title>A Reasonable Page Title</title>
		<meta name="description" content="This is a sufficiently long and descriptive meta description for the page.">
		<meta name="viewport" content="width=device-width, initial-scale=1">
		<link rel="canonical" href="https://example.com/">
		<meta property="og:title" content="A Reasonable Page Title">
	</head><body><h1>Heading</h1></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(seo.New().Analyze(context.Background(), res))

	for _, unwanted := range []string{"missing-title", "missing-meta-description", "missing-h1", "missing-canonical", "missing-viewport", "missing-opengraph", "missing-charset", "missing-lang"} {
		if got[unwanted] {
			t.Errorf("did not expect issue %q on a clean page", unwanted)
		}
	}
}

func TestSEONoindex(t *testing.T) {
	p := htmlPage(t, `<html><head><title>Page With Noindex</title><meta name="robots" content="noindex,follow"></head><body><h1>x</h1></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if !codes(seo.New().Analyze(context.Background(), res))["meta-noindex"] {
		t.Error("expected meta-noindex issue")
	}
}
