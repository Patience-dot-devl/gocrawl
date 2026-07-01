// Package diff compares two crawl reports and describes what changed between them: which
// issues are new, which were resolved, which persist, plus summary and page-set deltas.
//
// It is a pure transform over two *report.Report values — it never fetches, reads files, or
// mutates anything. The CLI's `compare` command and any future scheduled-comparison feature
// build on this seam, mirroring how the report and sitemap packages stay side-effect free.
package diff

import (
	"sort"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// Meta identifies one side of a comparison.
type Meta struct {
	Seed         string `json:"seed"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	PagesCrawled int    `json:"pages_crawled"`
}

func metaOf(r *report.Report) Meta {
	return Meta{
		Seed:         r.Seed,
		StartedAt:    r.StartedAt,
		FinishedAt:   r.FinishedAt,
		PagesCrawled: r.PagesCrawled,
	}
}

// Diff is the full comparison of a base report against a current one. "Base" is the earlier
// crawl and "current" the later one, so a resolved issue is present in base but gone in
// current, and a new issue is the reverse.
type Diff struct {
	Base    Meta        `json:"base"`
	Current Meta        `json:"current"`
	Issues  IssueDiff   `json:"issues"`
	Summary SummaryDiff `json:"summary"`
	Pages   PageDiff    `json:"pages"`
}

// IssueDiff buckets issues by how they changed between the two crawls. Issue identity is
// (analyzer, code, url); message, severity, and data may shift without changing identity, so
// the current-crawl copy is kept for New and Persisting and the base copy for Resolved.
type IssueDiff struct {
	New        []analyze.Issue `json:"new"`        // present now, absent before (regressions / freshly found)
	Resolved   []analyze.Issue `json:"resolved"`   // present before, absent now (fixed or no longer reached)
	Persisting []analyze.Issue `json:"persisting"` // present in both crawls
}

// SummaryDiff reports count deltas. Each map is keyed the same way as report.Summary and
// holds current-minus-base, so positive means "more now". Keys present in only one side
// appear with the signed full count.
type SummaryDiff struct {
	BySeverity map[string]int `json:"by_severity"`
	ByAnalyzer map[string]int `json:"by_analyzer"`
	ByStatus   map[string]int `json:"pages_by_status"`
	// NewBySeverity / ResolvedBySeverity tally the issues in IssueDiff.New / .Resolved by
	// severity, the headline "what got worse / better" numbers.
	NewBySeverity      map[string]int `json:"new_by_severity"`
	ResolvedBySeverity map[string]int `json:"resolved_by_severity"`
}

// PageDiff reports which crawled URLs appeared or disappeared between the two crawls, drawn
// from each report's site-map entries. Nil site maps yield empty slices.
type PageDiff struct {
	Added   []string `json:"added"`   // crawled now, not before
	Removed []string `json:"removed"` // crawled before, not now
}

// issueKey is the stable identity of a finding across crawls.
func issueKey(is analyze.Issue) [3]string {
	return [3]string{is.Analyzer, is.Code, is.URL}
}

// Compare diffs base (the earlier crawl) against current (the later one).
func Compare(base, current *report.Report) *Diff {
	d := &Diff{
		Base:    metaOf(base),
		Current: metaOf(current),
	}
	d.Issues = compareIssues(base.Issues, current.Issues)
	d.Summary = compareSummary(base, current, d.Issues)
	d.Pages = comparePages(base, current)
	return d
}

func compareIssues(baseIssues, currentIssues []analyze.Issue) IssueDiff {
	baseByKey := make(map[[3]string]analyze.Issue, len(baseIssues))
	for _, is := range baseIssues {
		baseByKey[issueKey(is)] = is
	}
	currentKeys := make(map[[3]string]bool, len(currentIssues))

	var d IssueDiff
	for _, is := range currentIssues {
		k := issueKey(is)
		currentKeys[k] = true
		if _, ok := baseByKey[k]; ok {
			d.Persisting = append(d.Persisting, is)
		} else {
			d.New = append(d.New, is)
		}
	}
	for _, is := range baseIssues {
		if !currentKeys[issueKey(is)] {
			d.Resolved = append(d.Resolved, is)
		}
	}
	sortIssues(d.New)
	sortIssues(d.Resolved)
	sortIssues(d.Persisting)
	return d
}

// sortIssues orders issues deterministically (severity worst-first, then analyzer/code/url)
// so reports and tests are stable regardless of crawl ordering.
func sortIssues(issues []analyze.Issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if r := severityRank(a.Severity) - severityRank(b.Severity); r != 0 {
			return r < 0
		}
		if a.Analyzer != b.Analyzer {
			return a.Analyzer < b.Analyzer
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.URL < b.URL
	})
}

func severityRank(s analyze.Severity) int {
	switch s {
	case analyze.Error:
		return 0
	case analyze.Warning:
		return 1
	default:
		return 2
	}
}

func compareSummary(base, current *report.Report, id IssueDiff) SummaryDiff {
	return SummaryDiff{
		BySeverity:         deltaMap(base.Summary.BySeverity, current.Summary.BySeverity),
		ByAnalyzer:         deltaMap(base.Summary.ByAnalyzer, current.Summary.ByAnalyzer),
		ByStatus:           deltaMap(base.Summary.ByStatus, current.Summary.ByStatus),
		NewBySeverity:      countBySeverity(id.New),
		ResolvedBySeverity: countBySeverity(id.Resolved),
	}
}

// deltaMap returns current-minus-base for every key present in either map, dropping zero
// deltas so the result only carries what changed.
func deltaMap(base, current map[string]int) map[string]int {
	out := make(map[string]int)
	for k, v := range current {
		out[k] = v
	}
	for k, v := range base {
		out[k] -= v
	}
	for k, v := range out {
		if v == 0 {
			delete(out, k)
		}
	}
	return out
}

func countBySeverity(issues []analyze.Issue) map[string]int {
	out := map[string]int{}
	for _, is := range issues {
		out[string(is.Severity)]++
	}
	return out
}

func comparePages(base, current *report.Report) PageDiff {
	baseURLs := pageURLs(base)
	currentURLs := pageURLs(current)
	var d PageDiff
	for u := range currentURLs {
		if !baseURLs[u] {
			d.Added = append(d.Added, u)
		}
	}
	for u := range baseURLs {
		if !currentURLs[u] {
			d.Removed = append(d.Removed, u)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Removed)
	return d
}

// pageURLs is the set of crawled URLs in a report, taken from its site-map entries. A report
// produced by an older gocrawl without a site map yields an empty set (so page diffing is
// simply skipped rather than reporting every URL as removed/added).
func pageURLs(r *report.Report) map[string]bool {
	out := map[string]bool{}
	if r.SiteMap == nil {
		return out
	}
	for _, e := range r.SiteMap.Entries {
		out[e.Loc] = true
	}
	return out
}

// Unchanged reports whether nothing of substance differs between the two crawls.
func (d *Diff) Unchanged() bool {
	return len(d.Issues.New) == 0 && len(d.Issues.Resolved) == 0 &&
		len(d.Pages.Added) == 0 && len(d.Pages.Removed) == 0
}
