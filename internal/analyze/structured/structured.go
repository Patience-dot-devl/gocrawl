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
				Code: "structured-invalid-jsonld", Message: "JSON-LD block is not valid JSON",
				Data: map[string]any{"error": err.Error()},
			})
			return
		}
		types = append(types, collectTypes(v)...)
		for _, mr := range validateRequired(v) {
			issues = append(issues, analyze.Issue{
				Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
				Code: "structured-missing-required", Message: "Structured-data object is missing required schema.org fields",
				Data: map[string]any{"type": mr.typ, "missing": mr.missing},
			})
		}
	})

	if blocks == 0 {
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Info,
			Code: "structured-none", Message: "Page has no JSON-LD structured data",
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

// requiredFields lists the minimal required properties for common schema.org types. It is a
// pragmatic subset, not a full schema.org validator: enough to catch the most common
// structured-data mistakes (a typed object missing its headline name, etc.).
var requiredFields = map[string][]string{
	"Product":        {"name"},
	"Offer":          {"price"},
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

// missingReq records a typed object that is missing one or more required fields.
type missingReq struct {
	typ     string
	missing []string
}

// validateRequired walks a decoded JSON-LD value and reports typed objects (of a known type)
// that omit required fields, descending into @graph arrays the same way collectTypes does.
func validateRequired(v any) []missingReq {
	var out []missingReq
	switch t := v.(type) {
	case map[string]any:
		for _, ty := range asStrings(t["@type"]) {
			req, known := requiredFields[ty]
			if !known {
				continue
			}
			var missing []string
			for _, f := range req {
				if !hasField(t, f) {
					missing = append(missing, f)
				}
			}
			if len(missing) > 0 {
				out = append(out, missingReq{typ: ty, missing: missing})
			}
		}
		if g, ok := t["@graph"]; ok {
			out = append(out, validateRequired(g)...)
		}
	case []any:
		for _, item := range t {
			out = append(out, validateRequired(item)...)
		}
	}
	return out
}

// hasField reports whether m has a non-empty value for key f.
func hasField(m map[string]any, f string) bool {
	v, ok := m[f]
	if !ok {
		return false
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x) != ""
	case nil:
		return false
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
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
