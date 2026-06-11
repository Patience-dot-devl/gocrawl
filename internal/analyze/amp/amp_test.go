package amp_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/amp"
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

func TestAMPDetectedComplete(t *testing.T) {
	p := htmlPage(t, `<html amp><head>
		<link rel="canonical" href="https://example.com/article">
		<script async src="https://cdn.ampproject.org/v0.js"></script>
	</head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(amp.New().Analyze(context.Background(), res))
	if !got["amp-detected"] {
		t.Error("expected amp-detected")
	}
	if got["amp-missing-canonical"] {
		t.Error("did not expect amp-missing-canonical when canonical present")
	}
	if got["amp-missing-runtime"] {
		t.Error("did not expect amp-missing-runtime when runtime present")
	}
}

func TestAMPLightningAttr(t *testing.T) {
	p := htmlPage(t, `<html ⚡><head></head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if !codes(amp.New().Analyze(context.Background(), res))["amp-detected"] {
		t.Error("expected amp-detected for the lightning-emoji attribute")
	}
}

func TestAMPMissingMarkup(t *testing.T) {
	p := htmlPage(t, `<html amp><head></head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	got := codes(amp.New().Analyze(context.Background(), res))
	if !got["amp-missing-canonical"] {
		t.Error("expected amp-missing-canonical")
	}
	if !got["amp-missing-runtime"] {
		t.Error("expected amp-missing-runtime")
	}
}

func TestAMPNonAMPNoLink(t *testing.T) {
	p := htmlPage(t, `<html><head></head><body><p>plain page</p></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if len(amp.New().Analyze(context.Background(), res)) != 0 {
		t.Error("expected no issues on a non-AMP page without an amphtml link")
	}
}

func TestAMPNonAMPLinked(t *testing.T) {
	// With no crawl-engine index, result.Page cannot resolve the amphtml target, so the
	// analyzer emits the informational amp-amphtml-linked rather than amp-broken-amphtml.
	p := htmlPage(t, `<html><head>
		<link rel="amphtml" href="https://example.com/amp/article">
	</head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{p}}
	if !codes(amp.New().Analyze(context.Background(), res))["amp-amphtml-linked"] {
		t.Error("expected amp-amphtml-linked")
	}
}

func TestAMPBrokenAmphtml(t *testing.T) {
	mk := func(url string, status int, html string) *crawler.Page {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatalf("parse fixture: %v", err)
		}
		return &crawler.Page{RequestedURL: url, FinalURL: url, StatusCode: status, ContentType: "text/html", Doc: doc}
	}
	page := mk("https://example.com/article", 200,
		`<html><head><link rel="amphtml" href="https://example.com/amp/article"></head><body></body></html>`)
	ampPage := mk("https://example.com/amp/article", 404, `<html amp><head></head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{page, ampPage}}
	res.Reindex()
	if !codes(amp.New().Analyze(context.Background(), res))["amp-broken-amphtml"] {
		t.Error("expected amp-broken-amphtml when the amphtml target returns 404")
	}
}
