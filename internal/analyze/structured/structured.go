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
	issues := analyze.EachPage(result, a.analyzePage)
	return issues
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	g, parseErrs := schemaorg.Parse(p.Doc)

	var issues []analyze.Issue
	for _, e := range parseErrs {
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
			Code: "structured-invalid-jsonld", Message: "JSON-LD block is not valid JSON",
			Data: map[string]any{"error": e.Err},
		})
	}
	issues = append(issues, requiredIssues(p, g)...)
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

// requiredIssues reports typed nodes missing the fields their rich result requires. Task 4
// replaces this with the table-driven eligibility check in eligibility.go.
func requiredIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue {
	var issues []analyze.Issue
	for _, n := range g.Nodes {
		for _, ty := range n.Types {
			req, known := legacyRequired[ty]
			if !known {
				continue
			}
			var missing []string
			for _, f := range req {
				if !g.HasValue(n, f) {
					missing = append(missing, f)
				}
			}
			if len(missing) > 0 {
				issues = append(issues, analyze.Issue{
					Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
					Code: "structured-missing-required", Message: "Structured-data object is missing required schema.org fields",
					Data: map[string]any{"type": ty, "missing": missing},
				})
			}
		}
	}
	return issues
}

// legacyRequired is the pre-eligibility required-field table, carried unchanged except that
// Offer has left it — its fields are now reached through Product via a dotted path.
var legacyRequired = map[string][]string{
	"Product":        {"name"},
	"Article":        {"headline"},
	"NewsArticle":    {"headline"},
	"BlogPosting":    {"headline"},
	"Recipe":         {"name"},
	"Event":          {"name", "startDate"},
	"Organization":   {"name"},
	"LocalBusiness":  {"name"},
	"Person":         {"name"},
	"BreadcrumbList": {"itemListElement"},
	"FAQPage":        {"mainEntity"},
	"VideoObject":    {"name", "thumbnailUrl"},
}
