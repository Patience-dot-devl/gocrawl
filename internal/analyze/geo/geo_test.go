package geo_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/geo"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// fakeFetcher serves canned responses keyed by URL.
type fakeFetcher struct{ pages map[string]*crawler.Page }

func (f fakeFetcher) Fetch(_ context.Context, rawURL string) (*crawler.Page, error) {
	if p, ok := f.pages[rawURL]; ok {
		return p, nil
	}
	return &crawler.Page{RequestedURL: rawURL, StatusCode: 404}, nil
}

func doc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	d, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return d
}

// pageResult builds a single-page result with an empty seed so the /llms.txt fetch is skipped,
// isolating the per-page checks.
func pageResult(t *testing.T, html string) *crawler.Result {
	return &crawler.Result{Pages: []*crawler.Page{{FinalURL: "https://example.com/post", StatusCode: 200, ContentType: "text/html", Doc: doc(t, html)}}}
}

func run(t *testing.T, res *crawler.Result) []analyze.Issue {
	t.Helper()
	return geo.New(fakeFetcher{}).Analyze(context.Background(), res)
}

// runSpecialized runs with the opt-in quotable-density heuristic enabled.
func runSpecialized(t *testing.T, res *crawler.Result) []analyze.Issue {
	t.Helper()
	return geo.New(fakeFetcher{}, geo.WithQuotableDensity(true)).Analyze(context.Background(), res)
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func TestLlmsTxtFound(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/llms.txt": {StatusCode: 200, Body: []byte("# Example\n")},
	}}
	res := &crawler.Result{Seed: "https://example.com/"}
	if _, ok := find(geo.New(ff).Analyze(context.Background(), res), "geo-llms-txt"); !ok {
		t.Error("expected geo-llms-txt when /llms.txt returns 200")
	}
}

func TestLlmsTxtMissing(t *testing.T) {
	res := &crawler.Result{Seed: "https://example.com/"}
	if _, ok := find(geo.New(fakeFetcher{}).Analyze(context.Background(), res), "geo-no-llms-txt"); !ok {
		t.Error("expected geo-no-llms-txt when /llms.txt is absent")
	}
}

// TestLlmsTxtSoft404HTML covers servers that answer 200 with an HTML page for any unknown
// path: /llms.txt must not be reported as published just because the request returned 200.
func TestLlmsTxtSoft404HTML(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/llms.txt": {
			StatusCode: 200, ContentType: "text/html; charset=utf-8",
			Body: []byte("<!DOCTYPE html><html><body>Page not found</body></html>"),
		},
	}}
	res := &crawler.Result{Seed: "https://example.com/"}
	issues := geo.New(ff).Analyze(context.Background(), res)
	if _, ok := find(issues, "geo-llms-txt"); ok {
		t.Error("HTML soft-404 should not be reported as a published /llms.txt")
	}
	if _, ok := find(issues, "geo-no-llms-txt"); !ok {
		t.Error("expected geo-no-llms-txt for an HTML soft-404 at /llms.txt")
	}
}

// TestLlmsTxtRedirectedAway covers a /llms.txt request that redirects to the homepage.
func TestLlmsTxtRedirectedAway(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/llms.txt": {
			StatusCode: 200, FinalURL: "https://example.com/", Body: []byte("# Home\nWelcome"),
		},
	}}
	res := &crawler.Result{Seed: "https://example.com/"}
	if _, ok := find(geo.New(ff).Analyze(context.Background(), res), "geo-llms-txt"); ok {
		t.Error("a /llms.txt that redirects to the homepage should not count as published")
	}
}

func TestArticleMissingAuthorAndDate(t *testing.T) {
	issues := run(t, pageResult(t, `<html><body><article><h1>Title</h1><p>Body.</p></article></body></html>`))
	if _, ok := find(issues, "geo-missing-author"); !ok {
		t.Error("expected geo-missing-author for article with no author")
	}
	if _, ok := find(issues, "geo-missing-date"); !ok {
		t.Error("expected geo-missing-date for article with no date")
	}
}

func TestArticleWithAuthorAndDateFromJSONLD(t *testing.T) {
	issues := run(t, pageResult(t, `<html><head><script type="application/ld+json">
		{"@type":"BlogPosting","author":{"@type":"Person","name":"Ada"},"datePublished":"2026-01-01"}
	</script></head><body><h1>Title</h1><p>Body.</p></body></html>`))
	if _, ok := find(issues, "geo-missing-author"); ok {
		t.Error("geo-missing-author should not fire with JSON-LD author")
	}
	if _, ok := find(issues, "geo-missing-date"); ok {
		t.Error("geo-missing-date should not fire with JSON-LD datePublished")
	}
}

func TestArticleWithVisibleByline(t *testing.T) {
	issues := run(t, pageResult(t, `<html><head><meta name="author" content="Ada"></head>
		<body><article><time datetime="2026-01-01">Jan</time><p>Body.</p></article></body></html>`))
	if _, ok := find(issues, "geo-missing-author"); ok {
		t.Error("geo-missing-author should not fire with meta author")
	}
	if _, ok := find(issues, "geo-missing-date"); ok {
		t.Error("geo-missing-date should not fire with <time datetime>")
	}
}

func TestNonArticleNotChecked(t *testing.T) {
	issues := run(t, pageResult(t, `<html><body><main><p>Just a page.</p></main></body></html>`))
	if _, ok := find(issues, "geo-missing-author"); ok {
		t.Error("non-article page should not get geo-missing-author")
	}
}

func TestNoMainLandmark(t *testing.T) {
	prose := strings.Repeat("word ", 320)
	issues := run(t, pageResult(t, `<html><body><div><p>`+prose+`</p></div></body></html>`))
	if _, ok := find(issues, "geo-no-main-landmark"); !ok {
		t.Error("expected geo-no-main-landmark for content-heavy page with no landmark")
	}
}

func TestMainLandmarkPresent(t *testing.T) {
	prose := strings.Repeat("word ", 320)
	issues := run(t, pageResult(t, `<html><body><main><p>`+prose+`</p></main></body></html>`))
	if _, ok := find(issues, "geo-no-main-landmark"); ok {
		t.Error("geo-no-main-landmark should not fire when <main> is present")
	}
}

func TestThinPageNoLandmarkWarning(t *testing.T) {
	issues := run(t, pageResult(t, `<html><body><div><p>Short.</p></div></body></html>`))
	if _, ok := find(issues, "geo-no-main-landmark"); ok {
		t.Error("thin page should not trigger geo-no-main-landmark")
	}
}

func TestJSDependentContent(t *testing.T) {
	rendered := `<html><body><main><p>` + strings.Repeat("word ", 320) + `</p></main></body></html>`
	raw := `<html><body><div id="root"></div></body></html>`
	res := &crawler.Result{Pages: []*crawler.Page{{
		FinalURL: "https://example.com/post", StatusCode: 200, ContentType: "text/html",
		Doc: doc(t, rendered), RawBody: []byte(raw),
	}}}
	is, ok := find(run(t, res), "geo-js-dependent-content")
	if !ok {
		t.Fatal("expected geo-js-dependent-content when raw HTML lacks the rendered prose")
	}
	if rw, _ := is.Data["raw_words"].(int); rw != 0 {
		t.Errorf("expected raw_words 0, got %v", rw)
	}
}

func TestNotJSDependentWhenRawHasContent(t *testing.T) {
	html := `<html><body><main><p>` + strings.Repeat("word ", 320) + `</p></main></body></html>`
	res := &crawler.Result{Pages: []*crawler.Page{{
		FinalURL: "https://example.com/post", StatusCode: 200, ContentType: "text/html",
		Doc: doc(t, html), RawBody: []byte(html),
	}}}
	if _, ok := find(run(t, res), "geo-js-dependent-content"); ok {
		t.Error("geo-js-dependent-content should not fire when raw HTML already has the content")
	}
}

func TestJSDependentSkippedWithoutRawBody(t *testing.T) {
	// Raw crawl mode leaves RawBody nil; the check has nothing to compare against.
	issues := run(t, pageResult(t, `<html><body><main><p>`+strings.Repeat("word ", 320)+`</p></main></body></html>`))
	if _, ok := find(issues, "geo-js-dependent-content"); ok {
		t.Error("geo-js-dependent-content should not fire without RawBody")
	}
}

func TestLowQuotableDensity(t *testing.T) {
	issues := runSpecialized(t, pageResult(t, `<html><body><main><p>`+strings.Repeat("word ", 320)+`</p></main></body></html>`))
	if _, ok := find(issues, "geo-low-quotable-density"); !ok {
		t.Error("expected geo-low-quotable-density for content-heavy prose with no data points")
	}
}

func TestQuotableDensitySufficient(t *testing.T) {
	prose := strings.Repeat("In 2024 revenue rose 50% to $1200 across 30 markets. ", 30)
	issues := runSpecialized(t, pageResult(t, `<html><body><main><p>`+prose+`</p></main></body></html>`))
	if _, ok := find(issues, "geo-low-quotable-density"); ok {
		t.Error("geo-low-quotable-density should not fire when prose is rich in concrete data")
	}
}

func TestQuotableDensityOffByDefault(t *testing.T) {
	issues := run(t, pageResult(t, `<html><body><main><p>`+strings.Repeat("word ", 320)+`</p></main></body></html>`))
	if _, ok := find(issues, "geo-low-quotable-density"); ok {
		t.Error("geo-low-quotable-density is opt-in and must not fire without WithQuotableDensity")
	}
}
