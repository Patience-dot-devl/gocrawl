package sitemapgen_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/sitemapgen"
)

func htmlPage(t *testing.T, url, title string, status, depth int) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<html><head><title>" + title + "</title></head><body>x</body></html>"))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{FinalURL: url, StatusCode: status, ContentType: "text/html", Doc: doc, Depth: depth}
}

func sampleResult(t *testing.T) *crawler.Result {
	t.Helper()
	home := htmlPage(t, "https://example.com/", "Home", 200, 0)
	home.Header = http.Header{"Last-Modified": []string{"Wed, 21 Oct 2015 07:28:00 GMT"}}
	return &crawler.Result{
		Seed: "https://example.com/",
		Pages: []*crawler.Page{
			home,
			htmlPage(t, "https://example.com/blog/post-1", "Post 1", 200, 2),
			htmlPage(t, "https://example.com/blog/post-2", "Post 2", 200, 2),
			htmlPage(t, "https://example.com/about", "About", 200, 1),
			// Excluded: non-200, non-HTML, and off-site.
			htmlPage(t, "https://example.com/gone", "Gone", 404, 1),
			{FinalURL: "https://example.com/logo.png", StatusCode: 200, ContentType: "image/png"},
			htmlPage(t, "https://other.com/external", "External", 200, 1),
		},
	}
}

func TestGenerateFiltersToIndexableOnSite(t *testing.T) {
	m := sitemapgen.Generate(sampleResult(t), nil, time.Unix(0, 0).UTC())

	got := map[string]string{}
	for _, e := range m.Entries {
		got[e.Loc] = e.LastMod
	}
	want := []string{
		"https://example.com/",
		"https://example.com/about",
		"https://example.com/blog/post-1",
		"https://example.com/blog/post-2",
	}
	if len(got) != len(want) {
		t.Fatalf("entry count = %d, want %d (%v)", len(got), len(want), got)
	}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Errorf("missing entry %q", w)
		}
	}
	if got["https://example.com/"] != "2015-10-21" {
		t.Errorf("home lastmod = %q, want 2015-10-21", got["https://example.com/"])
	}
	if got["https://example.com/about"] != "" {
		t.Errorf("about lastmod should be empty (no header), got %q", got["https://example.com/about"])
	}
}

func TestGenerateBuildsTree(t *testing.T) {
	m := sitemapgen.Generate(sampleResult(t), nil, time.Unix(0, 0).UTC())

	if m.Root.URL != "https://example.com/" {
		t.Errorf("root URL = %q, want home page", m.Root.URL)
	}
	// Children should be the synthetic/crawled top-level segments: about, blog.
	labels := []string{}
	for _, c := range m.Root.Children {
		labels = append(labels, c.Label)
	}
	if strings.Join(labels, ",") != "about,blog" {
		t.Fatalf("root children = %v, want [about blog]", labels)
	}
	blog := m.Root.Children[1]
	if len(blog.Children) != 2 {
		t.Fatalf("blog should have 2 children, got %d", len(blog.Children))
	}
	// blog itself was never crawled directly, so it is a synthetic node.
	if blog.URL != "" {
		t.Errorf("blog node URL = %q, want empty (synthetic)", blog.URL)
	}
}

func TestWriteXML(t *testing.T) {
	m := sitemapgen.Generate(sampleResult(t), nil, time.Unix(0, 0).UTC())
	var buf bytes.Buffer
	if err := sitemapgen.WriteXML(&buf, m); err != nil {
		t.Fatalf("WriteXML: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "<?xml") {
		t.Error("missing XML header")
	}
	if !strings.Contains(out, `xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`) {
		t.Error("missing sitemap namespace")
	}
	if !strings.Contains(out, "<loc>https://example.com/blog/post-1</loc>") {
		t.Error("missing post-1 loc")
	}
	if !strings.Contains(out, "<lastmod>2015-10-21</lastmod>") {
		t.Error("missing lastmod on home page")
	}
	if strings.Contains(out, "other.com") {
		t.Error("off-site URL leaked into sitemap")
	}
}

func sampleIssues() []analyze.Issue {
	return []analyze.Issue{
		{Analyzer: "seo", URL: "https://example.com/blog/post-1", Severity: analyze.Error, Code: "missing-title", Message: "No title"},
		{Analyzer: "seo", URL: "https://example.com/blog/post-1", Severity: analyze.Warning, Code: "short-desc", Message: "Short description"},
		{Analyzer: "images", URL: "https://example.com/about/", Severity: analyze.Info, Code: "img-alt", Message: "Missing alt"},
		// Trailing-slash variant must still match the home page node.
		{Analyzer: "seo", URL: "https://example.com", Severity: analyze.Warning, Code: "h1-missing", Message: "No H1"},
		// Site-wide: the robots analyzer's per-host finding names no crawled page.
		{Analyzer: "robots", URL: "host example.com", Severity: analyze.Warning, Code: "no-robots", Message: "No robots.txt"},
	}
}

func nodeByPath(root *sitemapgen.Node, segs ...string) *sitemapgen.Node {
	n := root
	for _, s := range segs {
		var next *sitemapgen.Node
		for _, c := range n.Children {
			if c.Label == s {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

func TestIssuesAttachedToPages(t *testing.T) {
	m := sitemapgen.Generate(sampleResult(t), sampleIssues(), time.Unix(0, 0).UTC())

	post1 := nodeByPath(m.Root, "blog", "post-1")
	if post1 == nil {
		t.Fatal("post-1 node missing")
	}
	if post1.Counts.Error != 1 || post1.Counts.Warning != 1 {
		t.Errorf("post-1 counts = %+v, want 1 error 1 warning", post1.Counts)
	}
	// Worst-first ordering: error before warning.
	if len(post1.Issues) != 2 || post1.Issues[0].Severity != "error" {
		t.Errorf("post-1 issues not error-first: %+v", post1.Issues)
	}
	// Trailing-slash issue URL still matched the home (root) node.
	if m.Root.Counts.Warning != 1 {
		t.Errorf("home node should have 1 warning from the trailing-slash match, got %+v", m.Root.Counts)
	}
	// /about issue URL had a trailing slash; node URL did not — still matches.
	if about := nodeByPath(m.Root, "about"); about == nil || about.Counts.Info != 1 {
		t.Errorf("about node should have 1 info issue, got %+v", about)
	}
}

func TestSubtotalsAndSiteWide(t *testing.T) {
	m := sitemapgen.Generate(sampleResult(t), sampleIssues(), time.Unix(0, 0).UTC())

	// blog is synthetic (no issues of its own) but two of post-1's live beneath it.
	blog := nodeByPath(m.Root, "blog")
	if blog.Counts.Total() != 0 {
		t.Errorf("blog has no own issues, got %+v", blog.Counts)
	}
	if blog.Subtotal.Total() != 2 {
		t.Errorf("blog subtotal = %d, want 2", blog.Subtotal.Total())
	}
	// One issue names no crawled page -> SiteWide.
	if len(m.SiteWide) != 1 || m.SiteWide[0].Code != "no-robots" {
		t.Errorf("SiteWide = %+v, want the robots finding", m.SiteWide)
	}
	// Totals span every issue (tree + site-wide).
	if m.Totals.Total() != 5 {
		t.Errorf("Totals = %+v, want 5", m.Totals)
	}
}

func TestWriteHTMLShowsIssues(t *testing.T) {
	m := sitemapgen.Generate(sampleResult(t), sampleIssues(), time.Unix(0, 0).UTC())
	var buf bytes.Buffer
	if err := sitemapgen.WriteHTML(&buf, m); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"missing-title", "No title", "issue on this page", "Site-wide issues", "no-robots"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}

func TestWriteHTML(t *testing.T) {
	m := sitemapgen.Generate(sampleResult(t), nil, time.Unix(0, 0).UTC())
	var buf bytes.Buffer
	if err := sitemapgen.WriteHTML(&buf, m); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Error("not an HTML document")
	}
	if !strings.Contains(out, "Post 1") {
		t.Error("page title missing from tree")
	}
	if !strings.Contains(out, "https://example.com/blog/post-1") {
		t.Error("page link missing from tree")
	}
}
