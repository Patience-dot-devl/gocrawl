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
	if _, ok := issueFor(links.New().Analyze(context.Background(), res), "https://example.com/a", "link-broken"); !ok {
		t.Error("expected broken-link for an internal link to a 404")
	}
}

// TestLinksTrailingSlashNotRedirect guards the false positive that motivated the Resolved
// field: index normalization strips the trailing slash before fetching, so a link authored
// as "/b/" (the site's canonical form) is fetched as "/b" and 301s back to "/b/". That
// self-induced redirect must not be reported as the page pointing at a redirecting URL.
func TestLinksTrailingSlashNotRedirect(t *testing.T) {
	a := mkPage(t, "https://example.com/a", 200, `<html><body><a href="/b/">to b</a></body></html>`)
	a.Links = []crawler.Link{{
		URL:      "https://example.com/b",  // dedup form (slash stripped)
		Resolved: "https://example.com/b/", // authored form (slash preserved)
		Anchor:   "to b",
	}}
	// /b was requested in the stripped form and 301'd to its canonical trailing-slash URL.
	b := &crawler.Page{
		RequestedURL: "https://example.com/b",
		FinalURL:     "https://example.com/b/",
		StatusCode:   200,
		ContentType:  "text/html",
		Redirects:    []crawler.Redirect{{From: "https://example.com/b", To: "https://example.com/b/", Status: 301}},
	}
	res := &crawler.Result{Pages: []*crawler.Page{a, b}}
	res.Reindex()
	if is, ok := issueFor(links.New().Analyze(context.Background(), res), "https://example.com/a", "link-to-redirect"); ok {
		t.Errorf("trailing-slash-only redirect should not be flagged, got %+v", is.Data)
	}
}

// TestLinksGenuineRedirect ensures a real redirect (a path change, not just the slash) is
// still reported, with the authored target and final URL surfaced.
func TestLinksGenuineRedirect(t *testing.T) {
	a := mkPage(t, "https://example.com/a", 200, `<html><body><a href="/old">old</a></body></html>`)
	a.Links = []crawler.Link{{
		URL:      "https://example.com/old",
		Resolved: "https://example.com/old",
		Anchor:   "old",
	}}
	old := &crawler.Page{
		RequestedURL: "https://example.com/old",
		FinalURL:     "https://example.com/new",
		StatusCode:   200,
		ContentType:  "text/html",
		Redirects:    []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/new", Status: 301}},
	}
	res := &crawler.Result{Pages: []*crawler.Page{a, old}}
	res.Reindex()
	is, ok := issueFor(links.New().Analyze(context.Background(), res), "https://example.com/a", "link-to-redirect")
	if !ok {
		t.Fatal("expected link-to-redirect for a genuine path-changing redirect")
	}
	if is.Data["target"] != "https://example.com/old" || is.Data["final"] != "https://example.com/new" {
		t.Errorf("unexpected redirect data: %+v", is.Data)
	}
}

func TestLinksInboundCount(t *testing.T) {
	a := mkPage(t, "https://example.com/a", 200, `<html><body><a href="/b">Bee</a></body></html>`)
	a.Links = []crawler.Link{{URL: "https://example.com/b", Anchor: "Bee"}}
	b := mkPage(t, "https://example.com/b", 200, `<html><body>hi</body></html>`)
	res := &crawler.Result{Pages: []*crawler.Page{a, b}}
	res.Reindex()
	issues := links.New().Analyze(context.Background(), res)

	is, ok := issueFor(issues, "https://example.com/b", "link-inbound")
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
	if is, ok := issueFor(issues, "https://example.com/a", "link-inbound"); !ok || is.Data["count"] != 0 {
		t.Errorf("expected /a inbound-links with count 0, got %v (present=%v)", is.Data["count"], ok)
	}
}
