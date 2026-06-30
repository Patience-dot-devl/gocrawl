// Package duplicates implements cross-page duplicate detection: identical body content,
// titles, and meta descriptions across the crawl.
package duplicates

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer detects exact duplicate content, titles, and meta descriptions across pages.
type Analyzer struct{}

// New returns a new duplicates analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "duplicates" }
func (Analyzer) Description() string {
	return "Exact duplicate page content, titles, and meta descriptions across the crawl"
}

const maxListed = 10

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	contentGroups := map[string][]string{}
	titleGroups := map[string][]string{}
	descGroups := map[string][]string{}

	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		doc := p.Doc
		url := p.FinalURL

		// Body content hash.
		body := strings.ToLower(strings.Join(strings.Fields(doc.Find("body").Text()), " "))
		if body != "" {
			sum := md5.Sum([]byte(body))
			hash := hex.EncodeToString(sum[:])
			contentGroups[hash] = append(contentGroups[hash], url)
		}

		// Title.
		title := strings.TrimSpace(doc.Find("head title").Text())
		if title != "" {
			titleGroups[title] = append(titleGroups[title], url)
		}

		// Meta description.
		if desc, ok := doc.Find(`head meta[name="description"]`).Attr("content"); ok {
			desc = strings.TrimSpace(desc)
			if desc != "" {
				descGroups[desc] = append(descGroups[desc], url)
			}
		}
	}

	var issues []analyze.Issue

	// Duplicate content.
	for _, hash := range sortedKeys(contentGroups) {
		urls := distinctPages(contentGroups[hash])
		if len(urls) < 2 {
			continue
		}
		for _, url := range urls {
			issues = append(issues, analyze.Issue{
				Analyzer: "duplicates", URL: url, Severity: analyze.Warning,
				Code: "duplicate-content", Message: "Page body content is identical to other pages",
				Data: map[string]any{"duplicates": others(urls, url), "group_size": len(urls)},
			})
		}
	}

	// Duplicate titles.
	for _, title := range sortedKeys(titleGroups) {
		urls := distinctPages(titleGroups[title])
		if len(urls) < 2 {
			continue
		}
		for _, url := range urls {
			issues = append(issues, analyze.Issue{
				Analyzer: "duplicates", URL: url, Severity: analyze.Warning,
				Code: "duplicate-title", Message: "Page title is identical to other pages",
				Data: map[string]any{"title": title, "duplicates": others(urls, url), "group_size": len(urls)},
			})
		}
	}

	// Duplicate meta descriptions.
	for _, desc := range sortedKeys(descGroups) {
		urls := distinctPages(descGroups[desc])
		if len(urls) < 2 {
			continue
		}
		for _, url := range urls {
			issues = append(issues, analyze.Issue{
				Analyzer: "duplicates", URL: url, Severity: analyze.Info,
				Code: "duplicate-meta-description", Message: "Meta description is identical to other pages",
				Data: map[string]any{"duplicates": others(urls, url), "group_size": len(urls)},
			})
		}
	}

	return issues
}

// sortedKeys returns the map keys in sorted order.
func sortedKeys(groups map[string][]string) []string {
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// pageKey returns a URL's page identity for duplicate detection: everything but the query
// string and fragment. Two URLs that differ only by query parameters (e.g.
// ?solution=onboarding) or a #fragment address the same page, so they must not count as
// distinct pages duplicating each other's title, description, or content.
func pageKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	return u.String()
}

// distinctPages collapses URLs that address the same page (ignoring query string and
// fragment) to one representative each — preferring the bare URL with no query or fragment —
// and returns them sorted. This keeps query-string and anchor variants of a single page from
// registering as duplicates of one another while still surfacing genuinely distinct pages.
func distinctPages(urls []string) []string {
	rep := make(map[string]string, len(urls))
	for _, u := range urls {
		k := pageKey(u)
		if cur, ok := rep[k]; !ok || preferred(u, cur) {
			rep[k] = u
		}
	}
	out := make([]string, 0, len(rep))
	for _, v := range rep {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// preferred reports whether candidate is a better representative for a page than current: a
// bare URL (already equal to its page key) wins over one carrying a query/fragment, then the
// shorter, then the lexicographically smaller — so the choice is stable.
func preferred(candidate, current string) bool {
	cb, curb := candidate == pageKey(candidate), current == pageKey(current)
	if cb != curb {
		return cb
	}
	if len(candidate) != len(current) {
		return len(candidate) < len(current)
	}
	return candidate < current
}

// others returns up to maxListed URLs from urls, excluding self.
func others(urls []string, self string) []string {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if u == self {
			continue
		}
		out = append(out, u)
		if len(out) >= maxListed {
			break
		}
	}
	return out
}
