package diff

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/sitemapgen"
)

func issue(analyzer, code, url string, sev analyze.Severity) analyze.Issue {
	return analyze.Issue{Analyzer: analyzer, Code: code, URL: url, Severity: sev, Message: code}
}

func reportWith(seed string, issues []analyze.Issue, pages ...string) *report.Report {
	sum := report.Summary{BySeverity: map[string]int{}, ByAnalyzer: map[string]int{}, ByStatus: map[string]int{}}
	for _, is := range issues {
		sum.BySeverity[string(is.Severity)]++
		sum.ByAnalyzer[is.Analyzer]++
	}
	var entries []sitemapgen.Entry
	for _, p := range pages {
		entries = append(entries, sitemapgen.Entry{Loc: p})
	}
	return &report.Report{
		Seed:         seed,
		FinishedAt:   "2026-06-30T12:00:00Z",
		PagesCrawled: len(pages),
		Summary:      sum,
		Issues:       issues,
		SiteMap:      &sitemapgen.Map{Entries: entries},
	}
}

func TestCompareIssueBuckets(t *testing.T) {
	base := reportWith("https://x.test",
		[]analyze.Issue{
			issue("seo", "missing-title", "https://x.test/a", analyze.Error),   // resolved
			issue("links", "broken-link", "https://x.test/b", analyze.Warning), // persists
		},
		"https://x.test/a", "https://x.test/b",
	)
	current := reportWith("https://x.test",
		[]analyze.Issue{
			issue("links", "broken-link", "https://x.test/b", analyze.Warning), // persists
			issue("seo", "missing-meta", "https://x.test/c", analyze.Warning),  // new
		},
		"https://x.test/b", "https://x.test/c",
	)

	d := Compare(base, current)

	if got := len(d.Issues.New); got != 1 || d.Issues.New[0].Code != "missing-meta" {
		t.Fatalf("New = %+v, want 1 missing-meta", d.Issues.New)
	}
	if got := len(d.Issues.Resolved); got != 1 || d.Issues.Resolved[0].Code != "missing-title" {
		t.Fatalf("Resolved = %+v, want 1 missing-title", d.Issues.Resolved)
	}
	if got := len(d.Issues.Persisting); got != 1 || d.Issues.Persisting[0].Code != "broken-link" {
		t.Fatalf("Persisting = %+v, want 1 broken-link", d.Issues.Persisting)
	}
}

func TestCompareSeverityChangeIsNewAndResolved(t *testing.T) {
	// Same analyzer+code+url but severity changed: identity is unchanged, so it persists.
	base := reportWith("https://x.test", []analyze.Issue{issue("seo", "title-long", "https://x.test/a", analyze.Info)})
	current := reportWith("https://x.test", []analyze.Issue{issue("seo", "title-long", "https://x.test/a", analyze.Warning)})
	d := Compare(base, current)
	if len(d.Issues.Persisting) != 1 || len(d.Issues.New) != 0 || len(d.Issues.Resolved) != 0 {
		t.Fatalf("severity-only change should persist, got new=%d resolved=%d persisting=%d",
			len(d.Issues.New), len(d.Issues.Resolved), len(d.Issues.Persisting))
	}
}

func TestComparePagesAndSummaryDeltas(t *testing.T) {
	base := reportWith("https://x.test",
		[]analyze.Issue{issue("seo", "x", "https://x.test/a", analyze.Error)},
		"https://x.test/a")
	current := reportWith("https://x.test",
		[]analyze.Issue{
			issue("seo", "x", "https://x.test/a", analyze.Error),
			issue("seo", "y", "https://x.test/b", analyze.Error),
		},
		"https://x.test/a", "https://x.test/b")

	d := Compare(base, current)
	if len(d.Pages.Added) != 1 || d.Pages.Added[0] != "https://x.test/b" {
		t.Fatalf("Pages.Added = %v, want [b]", d.Pages.Added)
	}
	if len(d.Pages.Removed) != 0 {
		t.Fatalf("Pages.Removed = %v, want none", d.Pages.Removed)
	}
	if d.Summary.BySeverity["error"] != 1 {
		t.Fatalf("BySeverity[error] delta = %d, want 1", d.Summary.BySeverity["error"])
	}
	if d.Summary.NewBySeverity["error"] != 1 {
		t.Fatalf("NewBySeverity[error] = %d, want 1", d.Summary.NewBySeverity["error"])
	}
}

func TestUnchanged(t *testing.T) {
	r := reportWith("https://x.test",
		[]analyze.Issue{issue("seo", "x", "https://x.test/a", analyze.Error)},
		"https://x.test/a")
	d := Compare(r, r)
	if !d.Unchanged() {
		t.Fatalf("identical reports should be Unchanged")
	}
}

func TestNilSiteMapSkipsPageDiff(t *testing.T) {
	base := &report.Report{Seed: "https://x.test", Summary: report.Summary{BySeverity: map[string]int{}, ByAnalyzer: map[string]int{}, ByStatus: map[string]int{}}}
	current := &report.Report{Seed: "https://x.test", Summary: report.Summary{BySeverity: map[string]int{}, ByAnalyzer: map[string]int{}, ByStatus: map[string]int{}}}
	d := Compare(base, current)
	if len(d.Pages.Added) != 0 || len(d.Pages.Removed) != 0 {
		t.Fatalf("nil site maps should yield no page diff, got %+v", d.Pages)
	}
}

func TestTextReporter(t *testing.T) {
	base := reportWith("https://x.test",
		[]analyze.Issue{issue("seo", "missing-title", "https://x.test/a", analyze.Error)},
		"https://x.test/a")
	current := reportWith("https://x.test",
		[]analyze.Issue{issue("links", "broken-link", "https://x.test/b", analyze.Warning)},
		"https://x.test/b")
	d := Compare(base, current)

	var buf bytes.Buffer
	if err := (TextReporter{}).Write(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"1 warning new", "1 error resolved", "broken-link", "missing-title", "+ https://x.test/b"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q\n---\n%s", want, out)
		}
	}
}

func TestJSONReporterRoundTrips(t *testing.T) {
	base := reportWith("https://x.test", []analyze.Issue{issue("seo", "x", "https://x.test/a", analyze.Error)}, "https://x.test/a")
	current := reportWith("https://x.test", nil)
	d := Compare(base, current)

	var buf bytes.Buffer
	if err := (JSONReporter{}).Write(&buf, d); err != nil {
		t.Fatal(err)
	}
	var back Diff
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("diff JSON did not round-trip: %v", err)
	}
	if len(back.Issues.Resolved) != 1 {
		t.Fatalf("round-tripped diff lost resolved issues: %+v", back.Issues)
	}
}
