package analyze

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// SiteWideKey is the Data key a rolled-up issue carries. Its value is a map with "pages" (how
// many pages had the finding), "of" (the host's HTML pages) and "examples" (up to
// SiteWideExamples of those pages' URLs, sorted). It is a plain map, not a struct, so a report
// read back from JSON answers IsSiteWide the same way as a fresh one.
const SiteWideKey = "site_wide"

const (
	// SiteWideMinPages is the fewest pages an identical info finding must appear on to be
	// rolled up; below it, a per-page listing is short enough to read.
	SiteWideMinPages = 10
	// SiteWideExamples caps the example URLs a rolled-up issue lists.
	SiteWideExamples = 10
)

// RollupSiteWide collapses info findings that repeat across a host into one issue per host.
//
// A crawl repeats a host-wide condition (no CSP header, no Open Graph tags in the template, no
// tracking tags) on every page, which buried the few findings that matter under thousands of
// identical lines: a 20,000-page crawl produced 240,000 issues, all but 5 of them info. Here
// the issues that are the same statement (same analyzer, code, message and data) on the same
// host are grouped, and a group is rolled up when it covers at least SiteWideMinPages pages and
// at least half the host's HTML pages. The rolled-up issue keeps the code, message and data,
// names the host's root URL, says on how many pages it was found, and lists example pages
// under Data[SiteWideKey].
//
// Only info findings are rolled up: a warning or error is something to fix on that page, so
// it stays listed per page. Findings whose data differs page to page (link counts, response
// times) are different statements and also stay per page. Output order is kept: a rolled-up
// issue takes the place of its group's first member.
func RollupSiteWide(result *crawler.Result, issues []Issue) []Issue {
	htmlPages := htmlPagesByHost(result)

	type group struct {
		first int
		urls  map[string]bool
	}
	groups := map[string]*group{}
	keys := make([]string, len(issues))
	for i, is := range issues {
		if is.Severity != Info {
			continue
		}
		host, ok := hostOf(is.URL)
		if !ok {
			continue
		}
		data, err := json.Marshal(is.Data) // map keys marshal sorted, so equal data gives equal bytes
		if err != nil {
			continue
		}
		k := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", is.Analyzer, is.Code, host, is.Message, data)
		keys[i] = k
		g := groups[k]
		if g == nil {
			g = &group{first: i, urls: map[string]bool{}}
			groups[k] = g
		}
		g.urls[is.URL] = true
	}

	rolled := map[string]bool{}
	for k, g := range groups {
		host, _ := hostOf(issues[g.first].URL)
		of := htmlPages[host]
		if n := len(g.urls); n >= SiteWideMinPages && 2*n >= of {
			rolled[k] = true
		}
	}
	if len(rolled) == 0 {
		return issues
	}

	out := make([]Issue, 0, len(issues))
	for i, is := range issues {
		k := keys[i]
		if !rolled[k] {
			out = append(out, is)
			continue
		}
		g := groups[k]
		if g.first != i {
			continue
		}
		out = append(out, siteWideIssue(is, g.urls, htmlPages))
	}
	return out
}

// IsSiteWide reports whether is was produced by RollupSiteWide.
func IsSiteWide(is Issue) bool {
	_, ok := is.Data[SiteWideKey]
	return ok
}

func siteWideIssue(first Issue, urls map[string]bool, htmlPages map[string]int) Issue {
	u, _ := url.Parse(first.URL)
	host := u.Host
	of := htmlPages[host]
	if of < len(urls) {
		of = len(urls) // a finding on non-HTML pages; never claim more pages than exist
	}
	examples := make([]string, 0, len(urls))
	for p := range urls {
		examples = append(examples, p)
	}
	sort.Strings(examples)
	if len(examples) > SiteWideExamples {
		examples = examples[:SiteWideExamples]
	}

	data := make(map[string]any, len(first.Data)+1)
	for k, v := range first.Data {
		data[k] = v
	}
	data[SiteWideKey] = map[string]any{"pages": len(urls), "of": of, "examples": examples}

	is := first
	is.URL = u.Scheme + "://" + host + "/"
	is.Message = fmt.Sprintf("%s (on %d of %d pages)", first.Message, len(urls), of)
	is.Data = data
	return is
}

// htmlPagesByHost counts, per host, the crawled pages that parsed as HTML: the pages a
// per-page check could have looked at, which is what "of N pages" should mean.
func htmlPagesByHost(result *crawler.Result) map[string]int {
	n := map[string]int{}
	if result == nil {
		return n
	}
	for _, p := range result.Pages {
		if p.Doc == nil {
			continue
		}
		if host, ok := hostOf(p.FinalURL); ok {
			n[host]++
		}
	}
	return n
}

func hostOf(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	return u.Host, true
}
