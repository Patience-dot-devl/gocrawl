package main

import "testing"

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
