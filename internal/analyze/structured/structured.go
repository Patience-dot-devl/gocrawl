// Package structured extracts and reports on JSON-LD structured data (schema.org): which
// types a page declares, whether they carry the fields their rich results require, whether
// the markup contradicts itself or the page it sits on, and which pages look like they are
// missing markup they should have.
package structured

import (
	"context"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer extracts JSON-LD blocks and reports on the structured data they carry.
type Analyzer struct{}

// New returns a structured-data analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "structured" }
func (Analyzer) Description() string {
	return "JSON-LD structured data: schema.org type reporting, rich-result field eligibility, markup integrity, and missing-markup candidates"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	roll := newRollup()
	issues := analyze.EachPage(result, func(p *crawler.Page) []analyze.Issue {
		return a.analyzePage(p, roll)
	})
	return append(issues, roll.issues(analyze.SiteBase(result))...)
}

func (a Analyzer) analyzePage(p *crawler.Page, roll *rollup) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	g, parseErrs := schemaorg.Parse(p.Doc)

	var issues []analyze.Issue
	for _, e := range parseErrs {
		// error, not warning: the block is discarded whole. There is no judgement call
		// here — it either parses or it does not — so this carries no false-positive risk.
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Error,
			Code: "structured-invalid-jsonld", Message: "JSON-LD block is not valid JSON",
			Data: map[string]any{"error": e.Err},
		})
	}
	issues = append(issues, requiredIssues(p, g, roll)...)
	issues = append(issues, integrityIssues(p, g)...)
	issues = append(issues, candidateIssues(p, g)...)

	// A page with no parsed nodes and no failed blocks has no JSON-LD at all. Checking the
	// node count rather than the script count means a page whose only block is empty or
	// broken is reported through the more specific code, not as "none".
	if len(g.Nodes) == 0 && len(parseErrs) == 0 {
		return append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Info,
			Code: "structured-none", Message: "Page has no JSON-LD structured data",
		})
	}
	if types := g.Types(); len(types) > 0 {
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Info,
			Code: "structured-data", Message: "JSON-LD structured data found",
			Data: map[string]any{"types": types},
		})
	}
	return issues
}
