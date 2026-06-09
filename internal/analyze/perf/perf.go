// Package perf is a placeholder for Core Web Vitals analysis. Real LCP/CLS/INP/FCP/TTFB
// collection requires headless rendering (chromedp), which is on the roadmap. For now it
// reports response timing from the raw fetch and notes that full CWV needs headless mode.
package perf

import (
	"context"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer reports basic timing and flags that real Core Web Vitals are not yet collected.
type Analyzer struct{}

// New returns a performance analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "perf" }
func (Analyzer) Description() string {
	return "Performance/Core Web Vitals (stub — full CWV requires headless rendering)"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	issues := []analyze.Issue{{
		Analyzer: "perf", URL: result.Seed, Severity: analyze.Info,
		Code:    "cwv-not-collected",
		Message: "Core Web Vitals (LCP/CLS/INP/FCP/TTFB) require headless rendering, which is not yet implemented. Run with --render headless once available.",
	}}

	// Surface server response time (TTFB proxy) from the raw fetch as a useful interim signal.
	for _, p := range result.Pages {
		if p.StatusCode == 200 && p.Duration > 0 {
			issues = append(issues, analyze.Issue{
				Analyzer: "perf", URL: p.FinalURL, Severity: analyze.Info,
				Code: "response-time", Message: "Server response time (TTFB proxy)",
				Data: map[string]any{"duration_ms": p.Duration.Milliseconds()},
			})
		}
	}
	return issues
}
