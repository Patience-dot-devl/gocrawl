// Package seo implements technical on-page SEO checks (title, meta, canonical, headings,
// language, viewport, charset, social tags).
package seo

import (
	"context"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// Analyzer performs on-page technical SEO checks.
type Analyzer struct{}

// New returns a new SEO analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "seo" }
func (Analyzer) Description() string {
	return "On-page technical SEO: title, meta, canonical, headings, lang, viewport, charset, social tags"
}

const (
	titleMin = 10
	titleMax = 60
	descMin  = 50
	descMax  = 160
)

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, a.analyzePage)
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	url := p.FinalURL
	doc := p.Doc
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "seo", URL: url, Severity: sev, Code: code, Message: msg, Data: data})
	}

	// Title.
	title := strings.TrimSpace(doc.Find("head title").First().Text())
	switch {
	case title == "":
		add(analyze.Error, "seo-missing-title", "Page has no <title>", nil)
	case len(title) < titleMin:
		add(analyze.Warning, "seo-short-title", "Title is very short", map[string]any{"length": len(title), "title": title})
	case len(title) > titleMax:
		add(analyze.Warning, "seo-long-title", "Title may be truncated in SERPs", map[string]any{"length": len(title), "title": title})
	}

	// Meta description.
	desc, hasDesc := metaContent(doc, "name", "description")
	switch {
	case !hasDesc || strings.TrimSpace(desc) == "":
		add(analyze.Warning, "seo-missing-meta-description", "Page has no meta description", nil)
	case len(desc) < descMin:
		add(analyze.Info, "seo-short-meta-description", "Meta description is short", map[string]any{"length": len(desc)})
	case len(desc) > descMax:
		add(analyze.Info, "seo-long-meta-description", "Meta description may be truncated", map[string]any{"length": len(desc)})
	}

	// Meta robots noindex / nofollow.
	if robots, ok := metaContent(doc, "name", "robots"); ok {
		low := strings.ToLower(robots)
		if strings.Contains(low, "noindex") {
			add(analyze.Warning, "seo-meta-noindex", "Page is marked noindex", map[string]any{"robots": robots})
		}
		if strings.Contains(low, "nofollow") {
			add(analyze.Info, "seo-meta-nofollow", "Page is marked nofollow", map[string]any{"robots": robots})
		}
	}

	// X-Robots-Tag HTTP header directives (header-level equivalent of meta robots).
	if p.Header != nil {
		if xr := p.Header.Get("X-Robots-Tag"); xr != "" {
			low := strings.ToLower(xr)
			if strings.Contains(low, "noindex") {
				add(analyze.Warning, "seo-x-robots-noindex", "X-Robots-Tag header marks page noindex", map[string]any{"x_robots_tag": xr})
			}
			if strings.Contains(low, "nofollow") {
				add(analyze.Info, "seo-x-robots-nofollow", "X-Robots-Tag header marks page nofollow", map[string]any{"x_robots_tag": xr})
			}
		}
	}

	// Meta-refresh redirect (an HTTP 3xx redirect is preferred for SEO).
	if content, ok := metaHTTPEquiv(doc, "refresh"); ok && strings.TrimSpace(content) != "" {
		add(analyze.Warning, "seo-meta-refresh", "Page uses a meta-refresh redirect (prefer an HTTP 3xx)", map[string]any{"content": content})
	}

	// Canonical.
	if href, ok := doc.Find(`head link[rel="canonical"]`).First().Attr("href"); ok && strings.TrimSpace(href) != "" {
		if doc.Find(`head link[rel="canonical"]`).Length() > 1 {
			add(analyze.Warning, "seo-multiple-canonical", "Multiple canonical links found", nil)
		}
	} else {
		add(analyze.Info, "seo-missing-canonical", "Page has no canonical link", nil)
	}

	// Headings.
	h1 := doc.Find("h1")
	switch h1.Length() {
	case 0:
		add(analyze.Warning, "seo-missing-h1", "Page has no <h1>", nil)
	case 1:
	default:
		add(analyze.Info, "seo-multiple-h1", "Page has multiple <h1> elements", map[string]any{"count": h1.Length()})
	}

	// html lang.
	if lang, ok := doc.Find("html").First().Attr("lang"); !ok || strings.TrimSpace(lang) == "" {
		add(analyze.Info, "seo-missing-lang", "<html> has no lang attribute", nil)
	}

	// Viewport.
	if _, ok := metaContent(doc, "name", "viewport"); !ok {
		add(analyze.Info, "seo-missing-viewport", "Page has no viewport meta (mobile-friendliness)", nil)
	}

	// Charset.
	if doc.Find(`head meta[charset]`).Length() == 0 {
		if _, ok := metaHTTPEquiv(doc, "content-type"); !ok {
			add(analyze.Info, "seo-missing-charset", "Page declares no character set", nil)
		}
	}

	// Open Graph / Twitter cards.
	if doc.Find(`head meta[property^="og:"]`).Length() == 0 {
		add(analyze.Info, "seo-missing-opengraph", "No OpenGraph tags (affects social sharing)", nil)
	}

	return issues
}

func metaContent(doc *goquery.Document, attr, val string) (string, bool) {
	sel := doc.Find(`head meta[` + attr + `="` + val + `"]`).First()
	if sel.Length() == 0 {
		// Case-insensitive fallback.
		doc.Find("head meta").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if a, ok := s.Attr(attr); ok && strings.EqualFold(a, val) {
				sel = s
				return false
			}
			return true
		})
	}
	if sel.Length() == 0 {
		return "", false
	}
	c, ok := sel.Attr("content")
	return strings.TrimSpace(c), ok
}

func metaHTTPEquiv(doc *goquery.Document, val string) (string, bool) {
	return metaContent(doc, "http-equiv", val)
}
