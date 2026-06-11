// Package images implements image checks: missing alt text and missing
// width/height dimension attributes.
package images

import (
	"context"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// Analyzer performs image alt-text and dimension checks.
type Analyzer struct{}

// New returns a new images analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "images" }
func (Analyzer) Description() string {
	return "Image alt text and width/height dimension checks"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, a.analyzePage)
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	imgs := p.Doc.Find("img")
	if imgs.Length() == 0 {
		return nil
	}
	url := p.FinalURL
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "images", URL: url, Severity: sev, Code: code, Message: msg, Data: data})
	}

	var missingAlt, missingDim int
	var sample []string
	imgs.Each(func(_ int, s *goquery.Selection) {
		// An explicit empty alt="" is valid for decorative images; only flag
		// images with no alt attribute at all.
		if _, ok := s.Attr("alt"); !ok {
			missingAlt++
			if len(sample) < 5 {
				if src, ok := s.Attr("src"); ok {
					sample = append(sample, src)
				}
			}
		}
		_, hasW := s.Attr("width")
		_, hasH := s.Attr("height")
		if !hasW || !hasH {
			missingDim++
		}
	})

	if missingAlt > 0 {
		add(analyze.Warning, "img-missing-alt", "Images without an alt attribute", map[string]any{"count": missingAlt, "sample": sample})
	}
	if missingDim > 0 {
		add(analyze.Info, "img-missing-dimensions", "Images missing width or height attribute", map[string]any{"count": missingDim})
	}

	return issues
}
