package content_test

import (
	"strings"
	"testing"

	"context"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/content"
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

func codesFor(issues []analyze.Issue, url string) map[string]bool {
	out := map[string]bool{}
	for _, is := range issues {
		if is.URL == url {
			out[is.Code] = true
		}
	}
	return out
}

func body(words int) string {
	return `<html><head><title>T</title></head><body><p>` + strings.Repeat("word ", words) + `</p></body></html>`
}

func TestThinContent(t *testing.T) {
	res := &crawler.Result{Pages: []*crawler.Page{
		htmlPage(t, "https://example.com/thin", body(20)),
	}}
	got := codesFor(content.New().Analyze(context.Background(), res), "https://example.com/thin")
	if !got["thin-content"] {
		t.Error("expected thin-content issue")
	}
}

func TestLowContentBelowAverage(t *testing.T) {
	res := &crawler.Result{Pages: []*crawler.Page{
		htmlPage(t, "https://example.com/long1", body(400)),
		htmlPage(t, "https://example.com/long2", body(400)),
		htmlPage(t, "https://example.com/long3", body(400)),
		htmlPage(t, "https://example.com/low", body(120)),
	}}
	issues := content.New().Analyze(context.Background(), res)

	low := codesFor(issues, "https://example.com/low")
	if !low["low-content"] {
		t.Error("expected low-content on the below-average page")
	}
	if low["thin-content"] {
		t.Error("did not expect thin-content on a 120-word page")
	}

	normal := codesFor(issues, "https://example.com/long1")
	if normal["low-content"] || normal["thin-content"] {
		t.Error("did not expect any content issue on a normal page")
	}
}
