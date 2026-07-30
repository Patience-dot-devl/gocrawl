// Package utm audits UTM campaign tagging on a page's outbound links: partial tagging,
// empty or duplicated parameters, inconsistent key casing, and a per-page rollup. It is the
// first of the SEA (Search Engine Advertising) analyzers and operates purely on the links
// the crawler already extracted — it never fetches.
package utm

import (
	"context"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/seaurl"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Option configures the analyzer.
type Option func(*Analyzer)

// WithIgnoreExternalTagging controls whether the 4 per-link tagging-quality warnings
// (utm-partial-tagging, utm-empty-value, utm-duplicate-param, utm-inconsistent-casing) are
// suppressed for links leaving the crawled domain — the site owner doesn't control tagging on
// links it doesn't own (e.g. a third-party widget's own UTM-tagged badge link). On by default.
func WithIgnoreExternalTagging(on bool) Option { return func(a *Analyzer) { a.ignoreExternal = on } }

// Analyzer inspects outbound links for UTM campaign tagging.
type Analyzer struct {
	ignoreExternal bool
}

// New returns a UTM analyzer. External-link tagging-quality warnings are suppressed by
// default; pass WithIgnoreExternalTagging(false) to include them.
func New(opts ...Option) *Analyzer {
	a := &Analyzer{ignoreExternal: true}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (Analyzer) Name() string { return "utm" }
func (Analyzer) Description() string {
	return "UTM tagging audit on outbound links: partial/empty/duplicate params, casing, tagged-link summary"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, a.analyzePage)
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if len(p.Links) == 0 {
		return nil
	}
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "utm", URL: p.FinalURL, Severity: sev, Code: code, Message: msg, Data: data})
	}

	var tagged, externalTagged, internalTagged int
	for _, link := range p.Links {
		u := seaurl.Parse(link.URL)
		if !u.Tagged() {
			continue
		}
		tagged++
		if link.External {
			externalTagged++
		} else {
			internalTagged++
			add(analyze.Info, "utm-internal-tagged",
				"UTM-tagged link points to the same site (starts a new analytics session)",
				map[string]any{"target": link.URL})
		}

		if link.External && a.ignoreExternal {
			continue
		}

		present := u.PresentKeys()
		if hasAny(present, seaurl.RequiredUTMKeys) && !hasAll(present, seaurl.RequiredUTMKeys) {
			add(analyze.Warning, "utm-partial-tagging",
				"Link has some but not all of utm_source/utm_medium/utm_campaign",
				map[string]any{"target": link.URL, "present": present, "missing": u.Missing(seaurl.RequiredUTMKeys), "anchor": link.Anchor})
		}
		if len(u.Empty) > 0 {
			add(analyze.Warning, "utm-empty-value", "Link has UTM parameters with empty values",
				map[string]any{"target": link.URL, "keys": u.Empty})
		}
		if len(u.Duplicates) > 0 {
			add(analyze.Warning, "utm-duplicate-param", "Link repeats a UTM parameter",
				map[string]any{"target": link.URL, "keys": u.Duplicates})
		}
		if len(u.CasingMixed) > 0 {
			add(analyze.Info, "utm-inconsistent-casing", "UTM parameter keys are not lowercase (analytics tools are case-sensitive)",
				map[string]any{"target": link.URL, "keys": u.CasingMixed})
		}
	}

	add(analyze.Info, "utm-summary", "UTM tagging counts for this page's links",
		map[string]any{"total_links": len(p.Links), "tagged_links": tagged, "external_tagged": externalTagged, "internal_tagged": internalTagged})
	return issues
}

// hasAny reports whether any key in want is present in have.
func hasAny(have, want []string) bool {
	set := toSet(have)
	for _, w := range want {
		if set[w] {
			return true
		}
	}
	return false
}

// hasAll reports whether every key in want is present in have.
func hasAll(have, want []string) bool {
	set := toSet(have)
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

func toSet(in []string) map[string]bool {
	set := make(map[string]bool, len(in))
	for _, s := range in {
		set[s] = true
	}
	return set
}
