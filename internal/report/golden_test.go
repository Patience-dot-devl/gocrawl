package report_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/sitemapgen"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/report.golden.json from the current JSON reporter")

// goldenReport covers the shapes whose JSON encoding differs between encoding/json and
// encoding/json/v2 defaults: maps with several keys (key order), zero ints and false bools
// behind omitempty, characters that v1 escapes for HTML (<, >, &), non-ASCII text, nil and
// empty Data, and a site map with empty severity counts.
func goldenReport() *report.Report {
	startedAt := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	result := &crawler.Result{
		Seed:      "https://example.com",
		StartedAt: startedAt,
		Finished:  startedAt.Add(12 * time.Second),
		Pages: []*crawler.Page{
			{RequestedURL: "https://example.com/", FinalURL: "https://example.com/", StatusCode: 200, Doc: titledDoc("Home &amp; &lt;Welcome&gt;")},
			{RequestedURL: "https://example.com/blog/post", FinalURL: "https://example.com/blog/post", StatusCode: 200, Depth: 1, Doc: titledDoc("Café crème")},
			{RequestedURL: "https://example.com/missing", FinalURL: "https://example.com/missing", StatusCode: 404, Depth: 1},
			{RequestedURL: "https://example.com/quiet", FinalURL: "https://example.com/quiet", StatusCode: 200, Depth: 1},
		},
		Coverage: crawler.Coverage{
			Complete:         false,
			PageLimitReached: true,
			MaxPages:         4,
			SitemapTotal:     7,
			SitemapFetched:   3,
		},
	}
	issues := []analyze.Issue{
		{Analyzer: "seo", URL: "https://example.com/", Severity: analyze.Error, Code: "seo-missing-title", Message: "Page has no <title> & no <h1>"},
		{Analyzer: "seo", URL: "https://example.com/blog/post", Severity: analyze.Warning, Code: "seo-long-title", Message: "Title may be truncated", Data: map[string]any{"length": 73, "limit": 60, "title": "Café crème"}},
		{Analyzer: "links", URL: "https://example.com/", Severity: analyze.Error, Code: "links-broken", Message: "Broken link", Data: map[string]any{"target": "https://example.com/missing", "status": 404, "anchor": "Read <more>", "nofollow": false, "zero": 0}},
		{Analyzer: "redirects", URL: "https://example.com/old", Severity: analyze.Info, Code: "http-redirect", Message: "Page redirects", Data: map[string]any{}},
	}
	rep := report.Build(result, issues)
	sm := sitemapgen.Generate(result, issues, startedAt.Add(12*time.Second))
	rep.SiteMap = &sm
	return rep
}

// TestJSONReportGolden locks the exact bytes of the JSON report, the public contract that
// external tools parse. Any change to them, such as a move from encoding/json to
// encoding/json/v2, must show up here as a reviewed diff. Regenerate with:
//
//	go test ./internal/report -run TestJSONReportGolden -update-golden
func TestJSONReportGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := (report.JSONReporter{}).Write(&buf, goldenReport()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	path := filepath.Join("testdata", "report.golden.json")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file (run with -update-golden to create it): %v", err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("JSON report bytes changed; review the diff and rerun with -update-golden if intended.\n got:\n%s\nwant:\n%s", buf.Bytes(), want)
	}
}

func titledDoc(title string) *goquery.Document {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<html><head><title>" + title + "</title></head><body></body></html>"))
	if err != nil {
		panic(err)
	}
	return doc
}
