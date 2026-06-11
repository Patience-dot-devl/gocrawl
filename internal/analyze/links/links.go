// Package links implements link analysis: broken internal links, links to redirects,
// nofollow and external link reporting, and empty anchor detection.
package links

import (
	"context"
	"sort"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer inspects outbound links against the crawled page set.
type Analyzer struct{}

// New returns a links analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "links" }
func (Analyzer) Description() string {
	return "Broken internal links, links to redirects, nofollow/external counts, empty anchors, inbound link counts"
}

// maxAnchorSample caps how many distinct inbound anchor texts are reported per page.
const maxAnchorSample = 10

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	inbound := buildInbound(result)
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

	// Inbound internal link counts and anchor text, aggregated across the crawl.
	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		info := inbound[p]
		count, anchors := 0, []string{}
		if info != nil {
			count = info.count
			anchors = sampleAnchors(info.anchors)
		}
		issues = append(issues, analyze.Issue{
			Analyzer: "links", URL: p.FinalURL, Severity: analyze.Info,
			Code: "inbound-links", Message: "Internal inbound link count",
			Data: map[string]any{"count": count, "anchors": anchors},
		})
	}
	return issues
}

// inboundInfo accumulates inbound internal links pointing at a single page.
type inboundInfo struct {
	count   int
	anchors map[string]bool
}

// buildInbound tallies internal inbound links (and their anchor text) per crawled page by
// resolving every page's outbound links against the crawl set.
func buildInbound(result *crawler.Result) map[*crawler.Page]*inboundInfo {
	m := make(map[*crawler.Page]*inboundInfo)
	for _, p := range result.Pages {
		for _, link := range p.Links {
			if link.External {
				continue
			}
			target, ok := result.Page(link.URL)
			if !ok || target == p {
				continue
			}
			info := m[target]
			if info == nil {
				info = &inboundInfo{anchors: make(map[string]bool)}
				m[target] = info
			}
			info.count++
			if link.Anchor != "" {
				info.anchors[link.Anchor] = true
			}
		}
	}
	return m
}

// sampleAnchors returns up to maxAnchorSample distinct anchor texts in sorted order.
func sampleAnchors(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	sort.Strings(out)
	if len(out) > maxAnchorSample {
		out = out[:maxAnchorSample]
	}
	return out
}
