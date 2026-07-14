package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A minimal but complete JSON report, including a site-map tree, as `gocrawl crawl
// --format json` would write it.
const sampleReportJSON = `{
  "seed": "https://example.com",
  "started_at": "2026-06-30T10:00:00Z",
  "finished_at": "2026-06-30T10:01:00Z",
  "pages_crawled": 2,
  "summary": {"by_severity":{"error":1},"by_analyzer":{"seo":1},"pages_by_status":{"200":2}},
  "issues": [
    {"analyzer":"seo","url":"https://example.com/blog/post-1","severity":"error","code":"seo-missing-h1","message":"No H1 on page"}
  ],
  "site_map": {
    "seed":"https://example.com","host":"example.com","generated":"2026-06-30T10:01:00Z",
    "entries":[{"loc":"https://example.com/blog/post-1"}],
    "totals":{"error":1},
    "root":{"label":"/","url":"https://example.com","status":200,
      "children":[
        {"label":"blog","url":"https://example.com/blog","status":200,"subtotal":{"error":1},
          "children":[
            {"label":"post-1","url":"https://example.com/blog/post-1","title":"First post","status":200,"counts":{"error":1},"subtotal":{"error":1},
              "issues":[{"severity":"error","code":"seo-missing-h1","analyzer":"seo","message":"No H1 on page"}]}
          ]}
      ]}
  }
}`

func writeTempReport(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	if err := os.WriteFile(path, []byte(sampleReportJSON), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	return path
}

func TestRenderToHTML(t *testing.T) {
	in := writeTempReport(t)
	out := filepath.Join(t.TempDir(), "report.html")

	cmd := newRenderCmd()
	cmd.SetArgs([]string{in, "-o", out, "-f", "html"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("render: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	html := string(got)
	// Both tabs and the site-map chart (rebuilt from the JSON, no recrawl) must be present.
	for _, want := range []string{
		`id="tab-sitemap"`,    // site-map tab panel
		`ul class="sm-chart"`, // the org-chart container
		"post-1",              // a leaf node from the serialized tree
		"seo-missing-h1",      // its issue, shown in the node popover
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
}

func TestRenderToCSV(t *testing.T) {
	in := writeTempReport(t)
	out := filepath.Join(t.TempDir(), "report.csv")

	cmd := newRenderCmd()
	cmd.SetArgs([]string{in, "-o", out, "-f", "csv"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("render: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(got), "seo-missing-h1") {
		t.Errorf("CSV missing the issue row:\n%s", got)
	}
}

func TestRenderSitemapSideOutput(t *testing.T) {
	in := writeTempReport(t)
	htmlOut := filepath.Join(t.TempDir(), "report.html")
	xmlOut := filepath.Join(t.TempDir(), "sitemap.xml")

	cmd := newRenderCmd()
	cmd.SetArgs([]string{in, "-o", htmlOut, "--sitemap", xmlOut})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("render: %v", err)
	}
	got, err := os.ReadFile(xmlOut)
	if err != nil {
		t.Fatalf("read sitemap: %v", err)
	}
	if !strings.Contains(string(got), "https://example.com/blog/post-1") {
		t.Errorf("sitemap.xml missing the crawled URL:\n%s", got)
	}
}

// TestRenderResolvesStoreID guards against a real usability gap: `gocrawl history`'s help
// text claims its IDs work with `gocrawl render`, but render only accepted a literal file
// path. It must now resolve a stored crawl ID (and "latest") the same way `gocrawl compare`
// already does.
func TestRenderResolvesStoreID(t *testing.T) {
	storeDir := t.TempDir()
	saveViaCmd(t, storeDir, sampleReportJSON)

	out := filepath.Join(t.TempDir(), "report.html")
	cmd := newRenderCmd()
	cmd.SetArgs([]string{"latest", "-o", out, "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("render latest: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(got), "seo-missing-h1") {
		t.Errorf("rendered HTML missing the report's issue:\n%s", got)
	}
}

func TestRenderMissingFile(t *testing.T) {
	cmd := newRenderCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{filepath.Join(t.TempDir(), "nope.json")})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error for a missing report file")
	}
}

func TestRenderInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cmd := newRenderCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}
