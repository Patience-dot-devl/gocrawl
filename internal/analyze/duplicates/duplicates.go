// Package duplicates implements cross-page duplicate detection: identical body content,
// titles, and meta descriptions across the crawl.
package duplicates

import (
	"context"
	"crypto/md5"
	"encoding/hex"
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
	for _, urls := range sortedGroups(contentGroups) {
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
		urls := sortedURLs(titleGroups[title])
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
	for _, urls := range sortedGroups(descGroups) {
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

// sortedGroups returns the groups' URL slices, each sorted, ordered deterministically by key.
func sortedGroups(groups map[string][]string) [][]string {
	out := make([][]string, 0, len(groups))
	for _, k := range sortedKeys(groups) {
		out = append(out, sortedURLs(groups[k]))
	}
	return out
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

// sortedURLs returns a sorted copy of urls.
func sortedURLs(urls []string) []string {
	out := append([]string(nil), urls...)
	sort.Strings(out)
	return out
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
