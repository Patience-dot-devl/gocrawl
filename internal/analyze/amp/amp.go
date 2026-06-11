// Package amp implements AMP page detection, required-markup checks, and amphtml link
// validation against the crawled page set.
package amp

import (
	"context"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// Analyzer detects AMP pages and validates AMP markup and amphtml links.
type Analyzer struct{}

// New returns a new AMP analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "amp" }
func (Analyzer) Description() string {
	return "AMP page detection, required markup, and amphtml link validation"
}

const ampRuntimePrefix = "https://cdn.ampproject.org/v0.js"

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	var issues []analyze.Issue
	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		url := p.FinalURL
		doc := p.Doc
		add := func(sev analyze.Severity, code, msg string, data map[string]any) {
			issues = append(issues, analyze.Issue{Analyzer: "amp", URL: url, Severity: sev, Code: code, Message: msg, Data: data})
		}

		if isAMP(doc) {
			add(analyze.Info, "amp-detected", "Page is an AMP document", nil)

			if doc.Find(`head link[rel="canonical"]`).Length() == 0 {
				add(analyze.Warning, "amp-missing-canonical", "AMP page has no canonical link", nil)
			}

			if !hasAMPRuntime(doc) {
				add(analyze.Error, "amp-missing-runtime", "AMP page does not load the AMP runtime (v0.js)", nil)
			}
			continue
		}

		// Non-AMP page: validate any amphtml link.
		href, ok := doc.Find(`head link[rel="amphtml"]`).First().Attr("href")
		href = strings.TrimSpace(href)
		if !ok || href == "" {
			continue
		}
		if target, ok := result.Page(href); ok && (target.StatusCode >= 400 || len(target.Redirects) > 0) {
			add(analyze.Warning, "amp-broken-amphtml", "amphtml link points to a broken or redirecting target", map[string]any{"target": href})
		} else {
			add(analyze.Info, "amp-amphtml-linked", "Page links to an AMP version", map[string]any{"target": href})
		}
	}
	return issues
}

// isAMP reports whether the <html> element marks the document as AMP.
func isAMP(doc *goquery.Document) bool {
	html := doc.Find("html").First()
	if _, ok := html.Attr("amp"); ok {
		return true
	}
	_, ok := html.Attr("⚡")
	return ok
}

// hasAMPRuntime reports whether the page loads the AMP runtime script.
func hasAMPRuntime(doc *goquery.Document) bool {
	found := false
	doc.Find("script[src]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if src, ok := s.Attr("src"); ok && strings.HasPrefix(strings.TrimSpace(src), ampRuntimePrefix) {
			found = true
			return false
		}
		return true
	})
	return found
}
