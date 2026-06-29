package report_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

func fixtureReport() *report.Report {
	startedAt := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(7 * time.Second)
	result := &crawler.Result{
		Seed:      "https://example.com",
		StartedAt: startedAt,
		Finished:  finishedAt,
		Pages: []*crawler.Page{
			{FinalURL: "https://example.com/", StatusCode: 200},
			{FinalURL: "https://example.com/missing", StatusCode: 404},
		},
	}
	issues := []analyze.Issue{
		{Analyzer: "seo", URL: "https://example.com/", Severity: analyze.Error, Code: "missing-title", Message: "Page has no <title>"},
		{Analyzer: "seo", URL: "https://example.com/about", Severity: analyze.Warning, Code: "long-title", Message: "Title may be truncated", Data: map[string]any{"length": 73}},
		{Analyzer: "redirects", URL: "https://example.com/old", Severity: analyze.Info, Code: "redirect", Message: "Page redirects", Data: map[string]any{"to": "https://example.com/new", "status": 301}},
	}
	return report.Build(result, issues)
}

func TestFor(t *testing.T) {
	cases := []struct {
		format string
		want   report.Reporter
	}{
		{"json", report.JSONReporter{}},
		{"JSON", report.JSONReporter{}},
		{"", report.JSONReporter{}},
		{"xml", report.JSONReporter{}},
		{"csv", report.CSVReporter{}},
		{"CSV", report.CSVReporter{}},
		{"html", report.HTMLReporter{}},
		{"HTML", report.HTMLReporter{}},
	}
	for _, tc := range cases {
		got := report.For(tc.format)
		if got != tc.want {
			t.Errorf("For(%q) = %T, want %T", tc.format, got, tc.want)
		}
	}
}

func TestJSONReporterRoundTrip(t *testing.T) {
	r := fixtureReport()
	var buf bytes.Buffer
	if err := (report.JSONReporter{}).Write(&buf, r); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var got report.Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Seed != r.Seed {
		t.Errorf("seed: got %q want %q", got.Seed, r.Seed)
	}
	if len(got.Issues) != len(r.Issues) {
		t.Errorf("issues: got %d want %d", len(got.Issues), len(r.Issues))
	}
	if got.Summary.BySeverity["error"] != r.Summary.BySeverity["error"] {
		t.Errorf("severity error: got %d want %d", got.Summary.BySeverity["error"], r.Summary.BySeverity["error"])
	}
}

func TestCSVReporter(t *testing.T) {
	r := fixtureReport()
	var buf bytes.Buffer
	if err := (report.CSVReporter{}).Write(&buf, r); err != nil {
		t.Fatalf("Write: %v", err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(rows) != 1+len(r.Issues) {
		t.Fatalf("rows: got %d want %d", len(rows), 1+len(r.Issues))
	}
	wantHeader := []string{"analyzer", "severity", "code", "url", "message", "data"}
	for i, h := range wantHeader {
		if rows[0][i] != h {
			t.Errorf("header[%d]: got %q want %q", i, rows[0][i], h)
		}
	}
	if rows[1][0] != "seo" || rows[1][2] != "missing-title" {
		t.Errorf("first issue row: got %v", rows[1])
	}
}

func TestHTMLReporter(t *testing.T) {
	r := fixtureReport()
	var buf bytes.Buffer
	if err := (report.HTMLReporter{}).Write(&buf, r); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := buf.String()
	if _, err := html.Parse(strings.NewReader(out)); err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	for _, want := range []string{
		"https://example.com", // seed
		"missing-title",       // issue code
		"long-title",          // issue code
		"redirect",            // issue code
		"seo", "redirects",    // analyzer names
		"sev-error", "sev-warning", "sev-info", // severity classes
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

func TestHTMLReporterRendersSiteMapTab(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<html><head><title>Post One</title></head><body>x</body></html>"))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	startedAt := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	result := &crawler.Result{
		Seed:      "https://example.com",
		StartedAt: startedAt,
		Finished:  startedAt,
		Pages: []*crawler.Page{
			{FinalURL: "https://example.com/blog/post-1", StatusCode: 200, ContentType: "text/html", Doc: doc},
		},
	}
	issues := []analyze.Issue{
		{Analyzer: "seo", URL: "https://example.com/blog/post-1", Severity: analyze.Error, Code: "missing-h1", Message: "No H1 on page"},
		{Analyzer: "robots", URL: "host example.com", Severity: analyze.Warning, Code: "no-robots", Message: "No robots.txt"},
	}
	r := report.Build(result, issues)

	var buf bytes.Buffer
	if err := (report.HTMLReporter{}).Write(&buf, r); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := buf.String()
	if _, err := html.Parse(strings.NewReader(out)); err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	for _, want := range []string{
		`data-tab="sitemap"`, // the tab button
		`id="tab-sitemap"`,   // the tab panel
		"post-1",             // tree node label
		"Post One",           // page <title> in the tree
		"missing-h1",         // per-page issue code shown on its node
		"issue on this page", // the clickable issue disclosure
		"Site-wide issues",   // the host-level findings section
		"no-robots",          // the site-wide issue code
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML site-map tab missing %q", want)
		}
	}
}

func TestHTMLReporterRendersExplanations(t *testing.T) {
	r := fixtureReport()
	var buf bytes.Buffer
	if err := (report.HTMLReporter{}).Write(&buf, r); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"what this means",                   // explanation summary label
		"How to fix",                        // explanation field label
		"The page has no &lt;title&gt;",     // missing-title "What it is" text (HTML-escaped)
		"unique, descriptive &lt;title&gt;", // missing-title "How to fix" text
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing explanation text %q", want)
		}
	}
}

func TestHTMLReporterUnknownCodeHasNoExplanation(t *testing.T) {
	startedAt := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	result := &crawler.Result{
		Seed:      "https://example.com",
		StartedAt: startedAt,
		Finished:  startedAt,
		Pages:     []*crawler.Page{{FinalURL: "https://example.com/", StatusCode: 200}},
	}
	issues := []analyze.Issue{
		{Analyzer: "seo", URL: "https://example.com/", Severity: analyze.Info, Code: "totally-unknown-code", Message: "msg"},
	}
	var buf bytes.Buffer
	if err := (report.HTMLReporter{}).Write(&buf, report.Build(result, issues)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if strings.Contains(buf.String(), "what this means") {
		t.Error("unknown issue code should not render an explanation block")
	}
}

func TestHTMLReporterEscapesUntrustedContent(t *testing.T) {
	startedAt := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	result := &crawler.Result{
		Seed:      "https://example.com",
		StartedAt: startedAt,
		Finished:  startedAt,
		Pages:     []*crawler.Page{{FinalURL: "https://example.com/", StatusCode: 200}},
	}
	issues := []analyze.Issue{
		{Analyzer: "seo", URL: "https://example.com/", Severity: analyze.Warning, Code: "x", Message: `<script>alert("xss")</script>`},
	}
	r := report.Build(result, issues)

	var buf bytes.Buffer
	if err := (report.HTMLReporter{}).Write(&buf, r); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, `<script>alert("xss")</script>`) {
		t.Errorf("unescaped <script> tag found in HTML output")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("expected escaped &lt;script&gt; in HTML output, got: %s", out)
	}
}
