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

func TestCompareInstanceKeySeparatesSameKeyFindings(t *testing.T) {
	// Two site-wide rollups share analyzer, code and url and differ only by instance. Without
	// the instance in the identity, Product being fixed while VideoObject appears would read
	// as one persisting finding instead of one resolved and one new.
	rollup := func(typ string) analyze.Issue {
		is := issue("structured", "structured-missing-recommended", "https://x.test/", analyze.Info)
		is.Data = map[string]any{analyze.InstanceKey: typ}
		return is
	}
	base := reportWith("https://x.test", []analyze.Issue{rollup("Product"), rollup("Organization")})
	current := reportWith("https://x.test", []analyze.Issue{rollup("Organization"), rollup("VideoObject")})

	d := Compare(base, current)

	if len(d.Issues.Resolved) != 1 || d.Issues.Resolved[0].Data[analyze.InstanceKey] != "Product" {
		t.Errorf("Resolved = %+v, want only the Product rollup", d.Issues.Resolved)
	}
	if len(d.Issues.New) != 1 || d.Issues.New[0].Data[analyze.InstanceKey] != "VideoObject" {
		t.Errorf("New = %+v, want only the VideoObject rollup", d.Issues.New)
	}
	if len(d.Issues.Persisting) != 1 || d.Issues.Persisting[0].Data[analyze.InstanceKey] != "Organization" {
		t.Errorf("Persisting = %+v, want only the Organization rollup", d.Issues.Persisting)
	}
}

func TestCompareCountsDuplicateKeysAsMultiset(t *testing.T) {
	// Three identically keyed findings before and two after: one was fixed. Set-based pairing
	// reported all of them persisting and nothing resolved.
	broken := issue("links", "link-broken", "https://x.test/a", analyze.Error)
	base := reportWith("https://x.test", []analyze.Issue{broken, broken, broken})
	current := reportWith("https://x.test", []analyze.Issue{broken, broken})

	d := Compare(base, current)

	if len(d.Issues.Persisting) != 2 || len(d.Issues.Resolved) != 1 || len(d.Issues.New) != 0 {
		t.Errorf("got new=%d resolved=%d persisting=%d, want 0/1/2",
			len(d.Issues.New), len(d.Issues.Resolved), len(d.Issues.Persisting))
	}

	d = Compare(current, base)
	if len(d.Issues.Persisting) != 2 || len(d.Issues.New) != 1 || len(d.Issues.Resolved) != 0 {
		t.Errorf("reversed: got new=%d resolved=%d persisting=%d, want 1/0/2",
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
