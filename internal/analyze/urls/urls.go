// Package urls implements URL hygiene checks: uppercase letters, underscores,
// non-ASCII characters, and excessive length.
package urls

import (
	"context"
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

const maxURLLength = 115

// Analyzer performs URL hygiene checks.
type Analyzer struct{}

// New returns a new urls analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "urls" }
func (Analyzer) Description() string {
	return "URL hygiene: uppercase, underscores, non-ASCII, excessive length"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, a.analyzePage)
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if p.FinalURL == "" {
		return nil
	}
	final := p.FinalURL
	u, err := url.Parse(final)
	if err != nil {
		return nil
	}
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "urls", URL: final, Severity: sev, Code: code, Message: msg, Data: data})
	}

	path := u.Path
	if strings.ToLower(path) != path {
		add(analyze.Info, "url-uppercase", "URL path contains uppercase letters", map[string]any{"url": final})
	}
	if strings.Contains(path, "_") {
		add(analyze.Info, "url-underscore", "URL path contains underscores", map[string]any{"url": final})
	}
	if hasNonASCII(final) {
		add(analyze.Info, "url-non-ascii", "URL contains non-ASCII characters", map[string]any{"url": final})
	}
	if len(final) > maxURLLength {
		add(analyze.Info, "url-too-long", "URL is excessively long", map[string]any{"url": final, "length": len(final)})
	}

	return issues
}

func hasNonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return true
		}
	}
	return false
}
