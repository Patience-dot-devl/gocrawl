package redirectcheck_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func baseRule() redirectcheck.Rule {
	return redirectcheck.Rule{
		Original:            "/old",
		Target:              "/new",
		IgnoreTrailingSlash: "TRUE",
		IgnoreProtocol:      "TRUE",
		DisableIfPageExists: "TRUE",
	}
}

func containsSubstring(notes []string, substr string) bool {
	for _, n := range notes {
		if strings.Contains(n, substr) {
			return true
		}
	}
	return false
}

func TestCheckRuleCleanRedirect(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/new", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/new", Status: 301}}},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{"example.com/new": true})
	if res.Verdict != redirectcheck.VerdictOK {
		t.Fatalf("verdict = %q, notes = %v, want ok", res.Verdict, res.Notes)
	}
}

func TestCheckRuleWrongDestination(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/somewhere-else", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/somewhere-else", Status: 301}}},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{"example.com/new": true, "example.com/somewhere-else": true})
	if res.Verdict != redirectcheck.VerdictError {
		t.Fatalf("verdict = %q, notes = %v, want error", res.Verdict, res.Notes)
	}
	if !containsSubstring(res.Notes, "unexpected destination") {
		t.Errorf("notes = %v, want a note about unexpected destination", res.Notes)
	}
}

func TestCheckRule404Target(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/new", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/new", Status: 301}}},
		"https://example.com/new": {StatusCode: 404, FinalURL: "https://example.com/new"},
	}}
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{})
	if res.Verdict != redirectcheck.VerdictError {
		t.Fatalf("verdict = %q, notes = %v, want error", res.Verdict, res.Notes)
	}
	if !containsSubstring(res.Notes, "redirect target returns 404") {
		t.Errorf("notes = %v, want a note about the target returning 404", res.Notes)
	}
}

func TestCheckRuleLiveSourceDisableTrueIsWarning(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/old"},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	rule := baseRule()
	rule.DisableIfPageExists = "TRUE"
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", rule, map[string]bool{"example.com/new": true})
	if res.Verdict != redirectcheck.VerdictWarning {
		t.Fatalf("verdict = %q, notes = %v, want warning", res.Verdict, res.Notes)
	}
}

func TestCheckRuleLiveSourceDisableFalseIsError(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/old"},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	rule := baseRule()
	rule.DisableIfPageExists = "FALSE"
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", rule, map[string]bool{"example.com/new": true})
	if res.Verdict != redirectcheck.VerdictError {
		t.Fatalf("verdict = %q, notes = %v, want error", res.Verdict, res.Notes)
	}
}

func TestCheckRuleStaleSitemapEntry(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/new", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/new", Status: 301}}},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	// The old URL is still listed in the sitemap even though it now redirects away.
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{"example.com/old": true, "example.com/new": true})
	if res.Verdict != redirectcheck.VerdictWarning {
		t.Fatalf("verdict = %q, notes = %v, want warning", res.Verdict, res.Notes)
	}
	if !containsSubstring(res.Notes, "stale sitemap entry") {
		t.Errorf("notes = %v, want a stale-sitemap note", res.Notes)
	}
}

func TestCheckRuleTargetNotInSitemap(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/new", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/new", Status: 301}}},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{})
	if res.Verdict != redirectcheck.VerdictWarning {
		t.Fatalf("verdict = %q, notes = %v, want warning", res.Verdict, res.Notes)
	}
	if !containsSubstring(res.Notes, "target not confirmed in sitemap") {
		t.Errorf("notes = %v, want a target-not-in-sitemap note", res.Notes)
	}
}

func TestCheckRuleSourceFetchError(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {RequestedURL: "https://example.com/old", Err: "connection reset"},
	}}
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{})
	if res.Verdict != redirectcheck.VerdictError {
		t.Fatalf("verdict = %q, want error", res.Verdict)
	}
	if !containsSubstring(res.Notes, "connection reset") {
		t.Errorf("notes = %v, want the fetch error message", res.Notes)
	}
}
