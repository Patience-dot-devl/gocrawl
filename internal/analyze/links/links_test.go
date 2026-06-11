package links_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/links"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

func mkPage(t *testing.T, url string, status int, html string) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{RequestedURL: url, FinalURL: url, StatusCode: status, ContentType: "text/html", Doc: doc}
}

// issueFor returns the issue with the given URL and code, if present.
func issueFor(issues []analyze.Issue, url, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.URL == url && is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func TestLinksBrokenInternal(t *testing.T) {
	a := mkPage(t, "https://example.com/a", 200, `<html><body><a href="/b">to b</a></body></html>`)
	a.Links = []crawler.Link{{URL: "https://example.com/b", Anchor: "to b"}}
	b := mkPage(t, "https://example.com/b", 404, `<html><body>gone</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{a, b}}
	res.Reindex()
	if _, ok := issueFor(links.New().Analyze(context.Background(), res), "https://example.com/a", "broken-link"); !ok {
		t.Error("expected broken-link for an internal link to a 404")
	}
}

func TestLinksInboundCount(t *testing.T) {
	a := mkPage(t, "https://example.com/a", 200, `<html><body><a href="/b">Bee</a></body></html>`)
	a.Links = []crawler.Link{{URL: "https://example.com/b", Anchor: "Bee"}}
	b := mkPage(t, "https://example.com/b", 200, `<html><body>hi</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{a, b}}
	res.Reindex()
	issues := links.New().Analyze(context.Background(), res)

	is, ok := issueFor(issues, "https://example.com/b", "inbound-links")
	if !ok {
		t.Fatal("expected inbound-links issue for /b")
	}
	if is.Data["count"] != 1 {
		t.Errorf("expected inbound count 1 for /b, got %v", is.Data["count"])
	}
	anchors, _ := is.Data["anchors"].([]string)
	if len(anchors) != 1 || anchors[0] != "Bee" {
		t.Errorf("expected inbound anchor [Bee], got %v", anchors)
	}

	// /a has no inbound links → reported with count 0.
	if is, ok := issueFor(issues, "https://example.com/a", "inbound-links"); !ok || is.Data["count"] != 0 {
		t.Errorf("expected /a inbound-links with count 0, got %v (present=%v)", is.Data["count"], ok)
	}
}
