// Package content implements cross-page content analysis: thin content and pages whose
// word count falls well below the crawl-wide average.
package content

import (
	"context"
	"math"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer flags thin and below-average pages across the crawl.
type Analyzer struct{}

// New returns a new content analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "content" }
func (Analyzer) Description() string {
	return "Thin content and below-average word count detection"
}

const thinThreshold = 100

type pageWords struct {
	url   string
	words int
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	var pages []pageWords
	total := 0
	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		words := len(strings.Fields(p.Doc.Find("body").Text()))
		pages = append(pages, pageWords{url: p.FinalURL, words: words})
		total += words
	}

	var mean float64
	if len(pages) > 0 {
		mean = float64(total) / float64(len(pages))
	}
	roundedMean := int(math.Round(mean))

	var issues []analyze.Issue
	for _, pw := range pages {
		if pw.words < thinThreshold {
			issues = append(issues, analyze.Issue{
				Analyzer: "content", URL: pw.url, Severity: analyze.Warning,
				Code: "content-thin", Message: "Page has very little textual content",
				Data: map[string]any{"words": pw.words},
			})
			continue
		}
		if len(pages) >= 3 && mean > 0 && float64(pw.words) < mean*0.5 {
			issues = append(issues, analyze.Issue{
				Analyzer: "content", URL: pw.url, Severity: analyze.Info,
				Code: "content-low", Message: "Page word count is well below the site average",
				Data: map[string]any{"words": pw.words, "site_average": roundedMean},
			})
		}
	}
	return issues
}
