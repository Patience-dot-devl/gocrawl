package main

import (
	"reflect"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// The bare `gocrawl` command (which launches the interactive menu) must accept --user-agent so
// `gocrawl --user-agent endeavour-bot` pre-fills the menu instead of erroring on an unknown flag.
func TestRootAcceptsUserAgentFlag(t *testing.T) {
	root := newRootCmd()
	if err := root.ParseFlags([]string{"--user-agent", "endeavour-bot"}); err != nil {
		t.Fatalf("bare gocrawl should accept --user-agent, got: %v", err)
	}
	if ua, _ := root.Flags().GetString("user-agent"); ua != "endeavour-bot" {
		t.Errorf("user-agent = %q, want endeavour-bot", ua)
	}
}

// The crawl subcommand keeps its own --user-agent (the non-interactive path).
func TestCrawlStillHasUserAgentFlag(t *testing.T) {
	if newCrawlCmd().Flags().Lookup("user-agent") == nil {
		t.Error("gocrawl crawl lost its --user-agent flag")
	}
}

// TestAnalyzerSelectionDeselectAllRunsNone guards against a real bug: an empty Enabled list
// conventionally means "run all" analyzers, so deselecting every analyzer in the interactive
// form's multi-select silently ran all of them instead of none.
func TestAnalyzerSelectionDeselectAllRunsNone(t *testing.T) {
	all := []runner.AnalyzerInfo{{Name: "seo"}, {Name: "links"}, {Name: "robots"}}
	enabled, disabled := analyzerSelection(nil, all)
	if enabled != nil {
		t.Errorf("enabled = %v, want nil", enabled)
	}
	want := []string{"seo", "links", "robots"}
	if !reflect.DeepEqual(disabled, want) {
		t.Errorf("disabled = %v, want %v (every analyzer, so none run)", disabled, want)
	}
}

func TestAnalyzerSelectionSubsetSetsEnabled(t *testing.T) {
	all := []runner.AnalyzerInfo{{Name: "seo"}, {Name: "links"}, {Name: "robots"}}
	enabled, disabled := analyzerSelection([]string{"seo"}, all)
	if !reflect.DeepEqual(enabled, []string{"seo"}) {
		t.Errorf("enabled = %v, want [seo]", enabled)
	}
	if disabled != nil {
		t.Errorf("disabled = %v, want nil", disabled)
	}
}

func TestAnalyzerSelectionAllSelectedRunsAll(t *testing.T) {
	all := []runner.AnalyzerInfo{{Name: "seo"}, {Name: "links"}, {Name: "robots"}}
	enabled, disabled := analyzerSelection([]string{"seo", "links", "robots"}, all)
	if enabled != nil {
		t.Errorf("enabled = %v, want nil (run all)", enabled)
	}
	if disabled != nil {
		t.Errorf("disabled = %v, want nil", disabled)
	}
}
