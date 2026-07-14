package runner

import (
	"sort"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// reg builds the default registry. The fetcher is only used by analyzers at run time, not
// during selection, so a plain HTTP fetcher is fine here.
func reg() *analyze.Registry {
	return BuildRegistry(crawler.NewHTTPFetcher(crawler.DefaultOptions()), false)
}

func nameSet(as []analyze.Analyzer) map[string]bool { return names(as) }

func TestPlanAnalyzersStripQueryOff(t *testing.T) {
	got, skipped := planAnalyzers(reg(), config.AnalyzersConfig{}, false)
	if skipped != nil {
		t.Errorf("expected no skips with strip_query off, got %v", skipped)
	}
	for _, n := range queryDependentAnalyzers {
		if !nameSet(got)[n] {
			t.Errorf("query-dependent analyzer %q should run when strip_query is off", n)
		}
	}
}

func TestPlanAnalyzersStripQuerySkipsQueryAnalyzers(t *testing.T) {
	got, skipped := planAnalyzers(reg(), config.AnalyzersConfig{}, true)

	active := nameSet(got)
	for _, n := range queryDependentAnalyzers {
		if active[n] {
			t.Errorf("analyzer %q should be skipped when strip_query is on", n)
		}
	}
	// Unrelated analyzers still run.
	if !active["seo"] {
		t.Error("seo should still run when strip_query is on")
	}

	sort.Strings(skipped)
	want := append([]string{}, queryDependentAnalyzers...)
	sort.Strings(want)
	if len(skipped) != len(want) {
		t.Fatalf("skipped = %v, want %v", skipped, want)
	}
	for i := range want {
		if skipped[i] != want[i] {
			t.Fatalf("skipped = %v, want %v", skipped, want)
		}
	}
}

func TestPlanAnalyzersStripQueryOnlyReportsActiveSkips(t *testing.T) {
	// Allow-list excludes the query-dependent analyzers, so strip_query skips nothing.
	cfg := config.AnalyzersConfig{Enabled: []string{"seo", "links"}}
	got, skipped := planAnalyzers(reg(), cfg, true)
	if skipped != nil {
		t.Errorf("expected no skips when query-dependent analyzers are not selected, got %v", skipped)
	}
	if active := nameSet(got); !active["seo"] || !active["links"] {
		t.Errorf("expected seo and links to run, got %v", active)
	}
}

func TestPlanAnalyzersStripQuerySkipsExplicitlyEnabled(t *testing.T) {
	// A query-dependent analyzer explicitly enabled is still skipped under strip_query.
	cfg := config.AnalyzersConfig{Enabled: []string{"seo", "utm"}}
	got, skipped := planAnalyzers(reg(), cfg, true)
	if nameSet(got)["utm"] {
		t.Error("utm should be skipped under strip_query even when explicitly enabled")
	}
	if len(skipped) != 1 || skipped[0] != "utm" {
		t.Errorf("expected skipped=[utm], got %v", skipped)
	}
}

func TestCoverageNote(t *testing.T) {
	// Page limit → message names --max-pages and warns about incomplete findings.
	n := coverageNote(crawler.Coverage{DiscoveredNotCrawled: 12, PageLimitReached: true, MaxPages: 100})
	for _, want := range []string{"partial coverage", "12 in-scope", "--max-pages 100", "broken links"} {
		if !strings.Contains(n, want) {
			t.Errorf("page-limit note missing %q: %s", want, n)
		}
	}
	// Depth limit → message names --depth.
	if d := coverageNote(crawler.Coverage{DiscoveredNotCrawled: 3, DepthLimitReached: true, MaxDepth: 2}); !strings.Contains(d, "--depth 2") {
		t.Errorf("depth-limit note missing --depth: %s", d)
	}
	// Interrupted (e.g. Ctrl-C) → distinct message, not the limit-reached wording.
	i := coverageNote(crawler.Coverage{Interrupted: true})
	if !strings.Contains(i, "interrupted") {
		t.Errorf("interrupted note missing \"interrupted\": %s", i)
	}
	if strings.Contains(i, "--max-pages") || strings.Contains(i, "--depth") {
		t.Errorf("interrupted note shouldn't mention a limit flag: %s", i)
	}
}
