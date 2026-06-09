// Package links implements link analysis: broken internal links, links to redirects,
// nofollow and external link reporting, and empty anchor detection.
package links

import (
	"context"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer inspects outbound links against the crawled page set.
type Analyzer struct{}

// New returns a links analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "links" }
func (Analyzer) Description() string {
	return "Broken internal links, links to redirects, nofollow/external counts, empty anchors"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	var issues []analyze.Issue
	for _, p := range result.Pages {
		if len(p.Links) == 0 {
			continue
		}
		var external, nofollow, empty int
		for _, link := range p.Links {
			if link.External {
				external++
			}
			if link.Nofollow {
				nofollow++
			}
			if link.Anchor == "" {
				empty++
			}
			// Cross-reference internal links against the crawl to find broken targets.
			if !link.External {
				if target, ok := result.Page(link.URL); ok {
					switch {
					case target.StatusCode >= 400:
						issues = append(issues, analyze.Issue{
							Analyzer: "links", URL: p.FinalURL, Severity: analyze.Error,
							Code: "broken-link", Message: "Internal link points to an error page",
							Data: map[string]any{"target": link.URL, "status": target.StatusCode, "anchor": link.Anchor},
						})
					case len(target.Redirects) > 0:
						issues = append(issues, analyze.Issue{
							Analyzer: "links", URL: p.FinalURL, Severity: analyze.Warning,
							Code: "link-to-redirect", Message: "Internal link points to a redirecting URL",
							Data: map[string]any{"target": link.URL, "final": target.FinalURL},
						})
					}
				}
			}
		}
		if empty > 0 {
			issues = append(issues, analyze.Issue{
				Analyzer: "links", URL: p.FinalURL, Severity: analyze.Info,
				Code: "empty-anchor", Message: "Links with empty anchor text",
				Data: map[string]any{"count": empty},
			})
		}
		issues = append(issues, analyze.Issue{
			Analyzer: "links", URL: p.FinalURL, Severity: analyze.Info,
			Code: "link-summary", Message: "Link counts",
			Data: map[string]any{"total": len(p.Links), "external": external, "nofollow": nofollow},
		})
	}
	return issues
}
