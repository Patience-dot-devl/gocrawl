// Package pagination implements rel=next/prev pagination sequence detection and
// broken-target checks against the crawled page set.
package pagination

import (
	"context"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer detects rel=next/prev pagination links and validates their targets.
type Analyzer struct{}

// New returns a new pagination analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "pagination" }
func (Analyzer) Description() string {
	return "rel=next/prev pagination sequence detection and broken-target checks"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	var issues []analyze.Issue
	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		url := p.FinalURL
		doc := p.Doc

		nextHref, _ := doc.Find(`head link[rel="next"]`).First().Attr("href")
		prevHref, _ := doc.Find(`head link[rel="prev"]`).First().Attr("href")
		nextHref = strings.TrimSpace(nextHref)
		prevHref = strings.TrimSpace(prevHref)

		if nextHref == "" && prevHref == "" {
			continue
		}

		data := map[string]any{}
		if nextHref != "" {
			data["next"] = nextHref
		}
		if prevHref != "" {
			data["prev"] = prevHref
		}
		issues = append(issues, analyze.Issue{
			Analyzer: "pagination", URL: url, Severity: analyze.Info,
			Code: "pagination-detected", Message: "Page declares rel=next/prev pagination links",
			Data: data,
		})

		for _, rel := range []struct {
			name string
			href string
		}{{"next", nextHref}, {"prev", prevHref}} {
			if rel.href == "" {
				continue
			}
			target, ok := result.Page(rel.href)
			if !ok {
				continue
			}
			if target.StatusCode >= 400 || len(target.Redirects) > 0 {
				issues = append(issues, analyze.Issue{
					Analyzer: "pagination", URL: url, Severity: analyze.Warning,
					Code: "pagination-broken", Message: "Pagination link points to a broken or redirecting target",
					Data: map[string]any{"target": rel.href, "rel": rel.name, "status": target.StatusCode},
				})
			}
		}
	}
	return issues
}
