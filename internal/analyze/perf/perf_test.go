package perf

import (
	"context"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

func htmlPage(url string, render *crawler.RenderResult) *crawler.Page {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader("<html><body></body></html>"))
	return &crawler.Page{
		RequestedURL: url,
		FinalURL:     url,
		StatusCode:   200,
		ContentType:  "text/html",
		Doc:          doc,
		Render:       render,
	}
}

func codes(issues []analyze.Issue) []string {
	out := make([]string, len(issues))
	for i, is := range issues {
		out[i] = is.Code
	}
	return out
}

func contains(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

func TestAnalyzeGoodMetricsEmitOnlyMeasured(t *testing.T) {
	result := &crawler.Result{Seed: "https://example.com", Pages: []*crawler.Page{
		htmlPage("https://example.com/a", &crawler.RenderResult{
			Implemented: true, LCP: 1500, FCP: 1000, CLS: 0.05, TBT: 100, TTFB: 400,
		}),
	}}
	issues := New().Analyze(context.Background(), result)
	got := codes(issues)
	if len(got) != 1 || got[0] != "cwv-measured" {
		t.Fatalf("expected only cwv-measured, got %v", got)
	}
}

func TestAnalyzeNeedsImprovementAndPoorBands(t *testing.T) {
	result := &crawler.Result{Seed: "https://example.com", Pages: []*crawler.Page{
		htmlPage("https://example.com/ni", &crawler.RenderResult{
			Implemented: true,
			LCP:         3000, // needs-improvement (>2500, <=4000)
			FCP:         2500, // needs-improvement
			CLS:         0.2,  // needs-improvement
			TBT:         400,  // needs-improvement
			TTFB:        1200, // needs-improvement
		}),
		htmlPage("https://example.com/poor", &crawler.RenderResult{
			Implemented: true,
			LCP:         5000, // poor
			FCP:         3500, // poor
			CLS:         0.5,  // poor
			TBT:         900,  // poor
			TTFB:        2500, // poor
		}),
	}}
	got := codes(New().Analyze(context.Background(), result))

	wantNI := []string{
		"lcp-needs-improvement", "fcp-needs-improvement",
		"cls-needs-improvement", "tbt-needs-improvement", "ttfb-needs-improvement",
	}
	wantPoor := []string{
		"lcp-poor", "fcp-poor", "cls-poor", "tbt-poor", "ttfb-poor",
	}
	for _, c := range append(wantNI, wantPoor...) {
		if !contains(got, c) {
			t.Errorf("missing expected code %q in %v", c, got)
		}
	}
}

func TestAnalyzeRenderFailedEmitsInfo(t *testing.T) {
	result := &crawler.Result{Seed: "https://example.com", Pages: []*crawler.Page{
		htmlPage("https://example.com/fail", &crawler.RenderResult{
			Implemented: false, Note: "browser crashed",
		}),
	}}
	got := codes(New().Analyze(context.Background(), result))
	// No page rendered successfully, so this is the raw-fallback path.
	if !contains(got, "cwv-not-collected") {
		t.Errorf("expected raw fallback to emit cwv-not-collected, got %v", got)
	}
}

func TestAnalyzeMixedRenderedAndFailed(t *testing.T) {
	result := &crawler.Result{Seed: "https://example.com", Pages: []*crawler.Page{
		htmlPage("https://example.com/ok", &crawler.RenderResult{
			Implemented: true, LCP: 1500, FCP: 1000, CLS: 0.05, TBT: 100, TTFB: 400,
		}),
		htmlPage("https://example.com/fail", &crawler.RenderResult{
			Implemented: false, Note: "navigation timeout",
		}),
	}}
	got := codes(New().Analyze(context.Background(), result))
	if !contains(got, "cwv-measured") {
		t.Errorf("expected cwv-measured for the rendered page, got %v", got)
	}
	if !contains(got, "cwv-render-failed") {
		t.Errorf("expected cwv-render-failed for the failed page, got %v", got)
	}
}

func TestAnalyzeRawModeFallback(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader("<html><body></body></html>"))
	result := &crawler.Result{Seed: "https://example.com", Pages: []*crawler.Page{
		{
			RequestedURL: "https://example.com/", FinalURL: "https://example.com/",
			StatusCode: 200, ContentType: "text/html", Doc: doc,
			Duration: 1234 * 1000 * 1000, // 1234ms
		},
	}}
	got := codes(New().Analyze(context.Background(), result))
	if !contains(got, "cwv-not-collected") {
		t.Errorf("expected cwv-not-collected in raw mode, got %v", got)
	}
	if !contains(got, "response-time") {
		t.Errorf("expected response-time proxy in raw mode, got %v", got)
	}
}
