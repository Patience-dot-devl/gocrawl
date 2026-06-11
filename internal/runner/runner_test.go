package runner

import (
	"sort"
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
