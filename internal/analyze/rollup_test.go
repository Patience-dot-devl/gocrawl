package analyze

import (
	"fmt"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// siteOf returns a result with n HTML pages on host and the URLs of those pages.
func siteOf(t *testing.T, host string, n int) (*crawler.Result, []string) {
	t.Helper()
	r := &crawler.Result{Seed: "https://" + host + "/"}
	var urls []string
	for i := range n {
		u := fmt.Sprintf("https://%s/p%02d", host, i)
		r.Pages = append(r.Pages, page(t, u, "<html><body>x</body></html>"))
		urls = append(urls, u)
	}
	return r, urls
}

func repeated(urls []string, sev Severity, code string, data map[string]any) []Issue {
	var out []Issue
	for _, u := range urls {
		out = append(out, Issue{Analyzer: "seo", URL: u, Severity: sev, Code: code, Message: "Same finding", Data: data})
	}
	return out
}

func TestRollupSiteWideCollapsesRepeatedInfo(t *testing.T) {
	result, urls := siteOf(t, "example.com", 20)
	warn := Issue{Analyzer: "seo", URL: urls[3], Severity: Warning, Code: "seo-long-title", Message: "long"}
	issues := append([]Issue{warn}, repeated(urls[:12], Info, "seo-missing-viewport", nil)...)

	got := RollupSiteWide(result, issues)
	if len(got) != 2 {
		t.Fatalf("got %d issues, want the warning plus one rolled-up info: %+v", len(got), got)
	}
	if got[0].Code != "seo-long-title" {
		t.Errorf("order not kept: first issue is %s", got[0].Code)
	}
	r := got[1]
	if r.URL != "https://example.com/" || r.Code != "seo-missing-viewport" || r.Severity != Info {
		t.Errorf("rolled-up issue = %+v, want seo-missing-viewport on the host root", r)
	}
	if r.Message != "Same finding (on 12 of 20 pages)" {
		t.Errorf("message = %q", r.Message)
	}
	if !IsSiteWide(r) {
		t.Fatal("IsSiteWide = false for the rolled-up issue")
	}
	sw := r.Data[SiteWideKey].(map[string]any)
	if sw["pages"] != 12 || sw["of"] != 20 {
		t.Errorf("site_wide = %v, want pages 12 of 20", sw)
	}
	ex := sw["examples"].([]string)
	if len(ex) != SiteWideExamples || ex[0] != urls[0] {
		t.Errorf("examples = %v, want the first %d page URLs, sorted", ex, SiteWideExamples)
	}
}

func TestRollupSiteWideKeepsPerPageFindings(t *testing.T) {
	result, urls := siteOf(t, "example.com", 20)
	cases := map[string][]Issue{
		"warnings stay per page":       repeated(urls, Warning, "seo-missing-h1", nil),
		"fewer than the minimum pages": repeated(urls[:SiteWideMinPages-1], Info, "seo-missing-viewport", nil),
		"under half the host's pages":  repeated(urls[:9], Info, "seo-missing-viewport", nil),
	}
	// Per-page data that differs is a different statement on each page.
	var counts []Issue
	for i, u := range urls {
		counts = append(counts, Issue{Analyzer: "links", URL: u, Severity: Info, Code: "link-inbound", Message: "Internal inbound link count", Data: map[string]any{"count": i}})
	}
	cases["data differs per page"] = counts

	for name, issues := range cases {
		if got := RollupSiteWide(result, issues); len(got) != len(issues) {
			t.Errorf("%s: %d issues became %d, want them kept per page", name, len(issues), len(got))
		}
	}
	// Exactly half of 20 pages with the minimum met is rolled up.
	big, half := siteOf(t, "example.com", 2*SiteWideMinPages)
	if got := RollupSiteWide(big, repeated(half[:SiteWideMinPages], Info, "x", nil)); len(got) != 1 {
		t.Errorf("half the pages: got %d issues, want 1", len(got))
	}
}

func TestRollupSiteWideGroupsPerHostAndData(t *testing.T) {
	a, aURLs := siteOf(t, "a.example", 10)
	b, bURLs := siteOf(t, "b.example", 10)
	result := &crawler.Result{Pages: append(a.Pages, b.Pages...)}

	var issues []Issue
	issues = append(issues, repeated(aURLs, Info, "tracking-none", nil)...)
	issues = append(issues, repeated(bURLs, Info, "tracking-none", nil)...)
	// Same code, two different data values on one host: two statements, two groups.
	issues = append(issues, repeated(aURLs, Info, "geo", map[string]any{"v": 1})...)
	issues = append(issues, repeated(aURLs, Info, "geo", map[string]any{"v": 2})...)

	got := RollupSiteWide(result, issues)
	if len(got) != 4 {
		t.Fatalf("got %d issues, want one per host for tracking-none and one per data value for geo: %+v", len(got), got)
	}
	if got[0].URL != "https://a.example/" || got[1].URL != "https://b.example/" {
		t.Errorf("hosts = %s, %s", got[0].URL, got[1].URL)
	}
	if got[2].Data["v"] != 1 || got[3].Data["v"] != 2 {
		t.Errorf("original data not kept: %v, %v", got[2].Data, got[3].Data)
	}
}

func TestRollupSiteWideCountsDuplicatePerPageOnce(t *testing.T) {
	result, urls := siteOf(t, "example.com", 10)
	issues := append(repeated(urls, Info, "x", nil), repeated(urls, Info, "x", nil)...)
	got := RollupSiteWide(result, issues)
	if len(got) != 1 {
		t.Fatalf("got %d issues, want 1", len(got))
	}
	if sw := got[0].Data[SiteWideKey].(map[string]any); sw["pages"] != 10 {
		t.Errorf("pages = %v, want 10 distinct pages", sw["pages"])
	}
}
