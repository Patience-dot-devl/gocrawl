package seo_test

import (
	"context"
	"net/http"
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

	for _, want := range []string{"seo-missing-title", "seo-missing-meta-description", "seo-missing-h1", "seo-missing-canonical", "seo-missing-viewport", "seo-missing-opengraph"} {
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

	for _, unwanted := range []string{"seo-missing-title", "seo-missing-meta-description", "seo-missing-h1", "seo-missing-canonical", "seo-missing-viewport", "seo-missing-opengraph", "seo-missing-charset", "seo-missing-lang"} {
		if got[unwanted] {
			t.Errorf("did not expect issue %q on a clean page", unwanted)
		}
	}
}

func TestSEONoindex(t *testing.T) {
	p := htmlPage(t, `<html><head><title>Page With Noindex</title><meta name="robots" content="noindex,follow"></head><body><h1>x</h1></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if !codes(seo.New().Analyze(context.Background(), res))["seo-meta-noindex"] {
		t.Error("expected meta-noindex issue")
	}
}

func TestSEOSkippedHeadingLevel(t *testing.T) {
	p := htmlPage(t, `<html lang="en"><head><title>Skipped Heading Level Test</title></head>
		<body><h1>Title</h1><h3>Subsection</h3></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(seo.New().Analyze(context.Background(), res))
	if !got["seo-skipped-heading-level"] {
		t.Error("expected seo-skipped-heading-level issue")
	}
}

func TestSEONoSkippedHeadingLevel(t *testing.T) {
	p := htmlPage(t, `<html lang="en"><head><title>Sequential Heading Level Test</title></head>
		<body><h1>Title</h1><h2>Section</h2><h3>Subsection</h3></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(seo.New().Analyze(context.Background(), res))
	if got["seo-skipped-heading-level"] {
		t.Error("did not expect seo-skipped-heading-level issue")
	}
}

func TestSEOEmptyHeading(t *testing.T) {
	p := htmlPage(t, `<html lang="en"><head><title>Empty Heading Test</title></head>
		<body><h1>Title</h1><h2>   </h2></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(seo.New().Analyze(context.Background(), res))
	if !got["seo-empty-heading"] {
		t.Error("expected seo-empty-heading issue")
	}
}

func TestSEOXRobotsAndMetaRefresh(t *testing.T) {
	p := htmlPage(t, `<html><head><title>Header Robots And Refresh</title>
		<meta http-equiv="refresh" content="3;url=/elsewhere"></head><body><h1>x</h1></body></html>`)
	p.Header = http.Header{}
	p.Header.Set("X-Robots-Tag", "noindex, nofollow")
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(seo.New().Analyze(context.Background(), res))
	for _, want := range []string{"seo-x-robots-noindex", "seo-x-robots-nofollow", "seo-meta-refresh"} {
		if !got[want] {
			t.Errorf("expected issue %q, not found", want)
		}
	}
}

func TestSEOTitleLengthCountsRunesNotBytes(t *testing.T) {
	// 25 Japanese characters: 75 bytes (over the old byte-based 60 limit) but only 25
	// runes (comfortably within the 10-60 rune range), so this must not be flagged.
	title := strings.Repeat("日本語のタイトルです", 2) + "あいうえお" // 25 runes, 75 bytes
	p := htmlPage(t, `<html><head><title>`+title+`</title></head><body><h1>x</h1></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	issues := seo.New().Analyze(context.Background(), res)
	got := codes(issues)
	if got["seo-long-title"] {
		t.Error("multi-byte title under 60 runes incorrectly flagged as seo-long-title")
	}
	if got["seo-short-title"] {
		t.Error("multi-byte title over 10 runes incorrectly flagged as seo-short-title")
	}
}

func TestSEOLongASCIITitleStillFlagged(t *testing.T) {
	title := strings.Repeat("a", 61) // 61 runes == 61 bytes, over the 60-rune limit
	p := htmlPage(t, `<html><head><title>`+title+`</title></head><body><h1>x</h1></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	issues := seo.New().Analyze(context.Background(), res)
	got := codes(issues)
	if !got["seo-long-title"] {
		t.Error("expected a plain 61-character ASCII title to still be flagged as seo-long-title")
	}
	for _, is := range issues {
		if is.Code == "seo-long-title" {
			if length, _ := is.Data["length"].(int); length != 61 {
				t.Errorf("seo-long-title length = %v, want 61", is.Data["length"])
			}
		}
	}
}

func TestSEODescriptionLengthCountsRunesNotBytes(t *testing.T) {
	// 80 Japanese characters: 240 bytes (over the old byte-based 160 limit) but only 80
	// runes (within the 50-160 rune range), so this must not be flagged.
	desc := strings.Repeat("あ", 80)
	p := htmlPage(t, `<html><head><title>Some Fine Title Here</title>
		<meta name="description" content="`+desc+`"></head><body><h1>x</h1></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(seo.New().Analyze(context.Background(), res))
	if got["seo-long-meta-description"] {
		t.Error("multi-byte description under 160 runes incorrectly flagged as seo-long-meta-description")
	}
	if got["seo-short-meta-description"] {
		t.Error("multi-byte description over 50 runes incorrectly flagged as seo-short-meta-description")
	}
}
