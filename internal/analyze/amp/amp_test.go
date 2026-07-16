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

func mkPage(t *testing.T, url string, status int, html string) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{RequestedURL: url, FinalURL: url, StatusCode: status, ContentType: "text/html", Doc: doc}
}

func TestAMPBrokenAmphtml(t *testing.T) {
	page := mkPage(t, "https://example.com/article", 200,
		`<html><head><link rel="amphtml" href="https://example.com/amp/article"></head><body></body></html>`)
	ampPage := mkPage(t, "https://example.com/amp/article", 404, `<html amp><head></head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{page, ampPage}}
	res.Reindex()
	if !codes(amp.New().Analyze(context.Background(), res))["amp-broken-amphtml"] {
		t.Error("expected amp-broken-amphtml when the amphtml target returns 404")
	}
}

// TestAMPResolvesRelativeAmphtmlHref guards against a real bug: a relative amphtml href was
// passed straight to result.Page (an exact-match lookup), so it never matched the crawled
// page's absolute URL and a genuinely broken relative link went unreported.
func TestAMPResolvesRelativeAmphtmlHref(t *testing.T) {
	page := mkPage(t, "https://example.com/article", 200,
		`<html><head><link rel="amphtml" href="/amp/article"></head><body></body></html>`)
	ampPage := mkPage(t, "https://example.com/amp/article", 404, `<html amp><head></head><body></body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{page, ampPage}}
	res.Reindex()
	if !codes(amp.New().Analyze(context.Background(), res))["amp-broken-amphtml"] {
		t.Error("expected amp-broken-amphtml for a relative href resolving to a 404 target")
	}
}

// TestAMPTrailingSlashRedirectNotBroken guards against a false positive: an amphtml target
// that only redirects by a trailing slash (e.g. a canonical WordPress-style redirect) is not a
// genuinely broken link and must not be flagged.
func TestAMPTrailingSlashRedirectNotBroken(t *testing.T) {
	// The amphtml href is authored with the site's canonical trailing slash, matching where
	// the target actually ended up (FinalURL). The Redirects entry only exists because some
	// *other*, unrelated request reached it without the slash — irrelevant to this link.
	page := mkPage(t, "https://example.com/article", 200,
		`<html><head><link rel="amphtml" href="https://example.com/amp/article/"></head><body></body></html>`)
	ampPage := mkPage(t, "https://example.com/amp/article/", 200, `<html amp><head>
		<link rel="canonical" href="https://example.com/article">
		<script async src="https://cdn.ampproject.org/v0.js"></script>
	</head><body></body></html>`)
	ampPage.RequestedURL = "https://example.com/amp/article"
	ampPage.Redirects = []crawler.Redirect{{From: "https://example.com/amp/article", To: "https://example.com/amp/article/", Status: 301}}
	res := &crawler.Result{Pages: []*crawler.Page{page, ampPage}}
	res.Reindex()
	if codes(amp.New().Analyze(context.Background(), res))["amp-broken-amphtml"] {
		t.Error("a trailing-slash-only redirect should not be flagged amp-broken-amphtml")
	}
}
