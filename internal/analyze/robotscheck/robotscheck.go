// Package robotscheck reports on robots.txt: presence, declared sitemaps, and crawled
// URLs that violate disallow rules.
package robotscheck

import (
	"context"
	"net/url"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer inspects the robots.txt collected during the crawl.
type Analyzer struct{}

// New returns a robots.txt analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "robots" }
func (Analyzer) Description() string {
	return "robots.txt presence, declared sitemaps, and disallow-rule violations"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	var issues []analyze.Issue
	ua := result.Opts.UserAgent

	for host, data := range result.Robots {
		base := "host " + host
		if !data.Found {
			issues = append(issues, analyze.Issue{
				Analyzer: "robots", URL: base, Severity: analyze.Info,
				Code: "no-robots", Message: "No robots.txt found", Data: map[string]any{"status": data.Status},
			})
			continue
		}
		if len(data.Sitemaps) == 0 {
			issues = append(issues, analyze.Issue{
				Analyzer: "robots", URL: base, Severity: analyze.Info,
				Code: "no-sitemap-declared", Message: "robots.txt declares no Sitemap",
			})
		} else {
			issues = append(issues, analyze.Issue{
				Analyzer: "robots", URL: base, Severity: analyze.Info,
				Code: "sitemaps-declared", Message: "robots.txt declares sitemaps",
				Data: map[string]any{"sitemaps": data.Sitemaps},
			})
		}
	}

	// Flag crawled URLs that robots.txt disallows (only possible when --respect-robots was off).
	for _, p := range result.Pages {
		ref := p.FinalURL
		if ref == "" {
			ref = p.RequestedURL
		}
		u, err := url.Parse(ref)
		if err != nil {
			continue
		}
		data, ok := result.Robots[u.Host]
		if !ok || !data.Found {
			continue
		}
		path := u.Path
		if path == "" {
			path = "/"
		}
		if u.RawQuery != "" {
			path += "?" + u.RawQuery
		}
		if !data.TestAgent(path, ua) {
			issues = append(issues, analyze.Issue{
				Analyzer: "robots", URL: ref, Severity: analyze.Warning,
				Code: "crawled-disallowed", Message: "Crawled a URL disallowed by robots.txt",
			})
		}
	}

	return issues
}
