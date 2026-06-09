// Package structured extracts and reports on JSON-LD structured data (schema.org).
package structured

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// Analyzer extracts JSON-LD blocks and reports their schema.org types.
type Analyzer struct{}

// New returns a structured-data analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "structured" }
func (Analyzer) Description() string {
	return "JSON-LD structured data extraction and schema.org @type reporting"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, a.analyzePage)
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	var issues []analyze.Issue
	var types []string
	blocks := 0

	p.Doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		blocks++
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			issues = append(issues, analyze.Issue{
				Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
				Code: "invalid-jsonld", Message: "JSON-LD block is not valid JSON",
				Data: map[string]any{"error": err.Error()},
			})
			return
		}
		types = append(types, collectTypes(v)...)
	})

	if blocks == 0 {
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Info,
			Code: "no-structured-data", Message: "Page has no JSON-LD structured data",
		})
		return issues
	}
	if len(types) > 0 {
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Info,
			Code: "structured-data", Message: "JSON-LD structured data found",
			Data: map[string]any{"types": dedupe(types)},
		})
	}
	return issues
}

// collectTypes walks a decoded JSON-LD value collecting all @type values, descending into
// @graph arrays and nested objects.
func collectTypes(v any) []string {
	var out []string
	switch t := v.(type) {
	case map[string]any:
		if ty, ok := t["@type"]; ok {
			out = append(out, asStrings(ty)...)
		}
		if g, ok := t["@graph"]; ok {
			out = append(out, collectTypes(g)...)
		}
	case []any:
		for _, item := range t {
			out = append(out, collectTypes(item)...)
		}
	}
	return out
}

func asStrings(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
