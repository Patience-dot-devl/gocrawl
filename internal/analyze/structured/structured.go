// Package structured extracts and reports on JSON-LD structured data (schema.org).
package structured

import (
	"context"
	"encoding/json"
	"regexp"
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

	issues = append(issues, candidateIssues(p, toSet(types))...)

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

// articleTypes are the schema.org types that already cover an article-shaped page; a
// candidate suggestion only fires when none of them are present.
var articleTypes = map[string]bool{
	"Article": true, "NewsArticle": true, "BlogPosting": true, "TechArticle": true, "Report": true,
}

// minArticleWords gates the article candidate check on substantial body copy, so a small
// "News" nav label or an <article>-wrapped teaser doesn't trigger a false positive.
const minArticleWords = 150

// priceRe matches an on-page price such as "$19.99" or "29.00 EUR".
var priceRe = regexp.MustCompile(`[$€£¥]\s?\d[\d,.]*\d|\b\d[\d,.]*\d\s?(?:USD|EUR|GBP)\b`)

// cartSignalRe matches common calls-to-action that accompany a purchasable item.
var cartSignalRe = regexp.MustCompile(`(?i)add.?to.?cart|buy now|add.?to.?bag|add.?to.?basket`)

// videoHostRe matches iframe embeds from the video platforms most pages use instead of a
// native <video> element.
var videoHostRe = regexp.MustCompile(`(?i)youtube(-nocookie)?\.com|vimeo\.com`)

// candidateIssues runs low-noise heuristics that flag content shaped like a common schema.org
// type (breadcrumbs, a product, an article, an embedded video) whose matching JSON-LD is
// absent from types. Each check requires a reasonably specific on-page signal so the
// suggestion stays actionable rather than firing on every page.
func candidateIssues(p *crawler.Page, types map[string]bool) []analyze.Issue {
	doc := p.Doc
	var issues []analyze.Issue
	add := func(code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning, Code: code, Message: msg, Data: data,
		})
	}

	if !types["BreadcrumbList"] {
		if links, ok := hasBreadcrumbMarkup(doc); ok {
			add("structured-breadcrumb-candidate", "Page has breadcrumb navigation but no BreadcrumbList structured data",
				map[string]any{"links": links})
		}
	}

	if !types["Product"] && !types["Offer"] {
		if signal, ok := hasProductSignal(doc); ok {
			add("structured-product-candidate", "Page reads like a product page but has no Product structured data",
				map[string]any{"signal": signal})
		}
	}

	hasArticleType := false
	for t := range articleTypes {
		if types[t] {
			hasArticleType = true
			break
		}
	}
	if !hasArticleType {
		if words, ok := hasArticleSignal(doc); ok {
			add("structured-article-candidate", "Page reads like an article but has no Article/NewsArticle/BlogPosting structured data",
				map[string]any{"words": words})
		}
	}

	if !types["VideoObject"] {
		if src, ok := findVideoEmbed(doc); ok {
			add("structured-video-candidate", "Page embeds a video but has no VideoObject structured data",
				map[string]any{"src": src})
		}
	}

	return issues
}

// hasBreadcrumbMarkup reports whether the page has a breadcrumb-styled nav/list (identified
// by the "breadcrumb" convention in its class or aria-label) with at least two links, and how
// many links it contains.
func hasBreadcrumbMarkup(doc *goquery.Document) (int, bool) {
	links, found := 0, false
	doc.Find("nav, ol, ul").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		class, _ := s.Attr("class")
		aria, _ := s.Attr("aria-label")
		if !strings.Contains(strings.ToLower(class+" "+aria), "breadcrumb") {
			return true
		}
		if n := s.Find("a").Length(); n >= 2 {
			links, found = n, true
			return false
		}
		return true
	})
	return links, found
}

// hasProductSignal reports whether the page shows a purchasable-item signal: leftover
// Product/price microdata, or an on-page price next to a cart/buy call-to-action.
func hasProductSignal(doc *goquery.Document) (string, bool) {
	if doc.Find(`[itemprop="price"], [itemtype*="schema.org/Product"], [itemtype*="schema.org/Offer"]`).Length() > 0 {
		return "microdata", true
	}
	text := doc.Find("body").Text()
	if price := priceRe.FindString(text); price != "" && cartSignalRe.MatchString(text) {
		return price, true
	}
	return "", false
}

// hasArticleSignal reports whether the page has a substantial <article> with an author or
// publish-date signal, and the article's word count.
func hasArticleSignal(doc *goquery.Document) (int, bool) {
	content := doc.Find("article").First()
	if content.Length() == 0 {
		return 0, false
	}
	words := len(strings.Fields(content.Text()))
	if words < minArticleWords {
		return 0, false
	}
	hasAuthor := doc.Find(`[rel="author"], [itemprop="author"], meta[property="article:author"]`).Length() > 0
	hasDate := doc.Find(`time[datetime], meta[property="article:published_time"], meta[property="article:modified_time"]`).Length() > 0
	if !hasAuthor && !hasDate {
		return 0, false
	}
	return words, true
}

// findVideoEmbed returns the source of a native <video> element or a YouTube/Vimeo iframe
// embed, if either is present.
func findVideoEmbed(doc *goquery.Document) (string, bool) {
	if v := doc.Find("video").First(); v.Length() > 0 {
		if src, ok := v.Attr("src"); ok && src != "" {
			return src, true
		}
		if src, ok := v.Find("source").First().Attr("src"); ok && src != "" {
			return src, true
		}
		return "video", true
	}
	src, found := "", false
	doc.Find("iframe").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		iframeSrc, _ := s.Attr("src")
		if videoHostRe.MatchString(iframeSrc) {
			src, found = iframeSrc, true
			return false
		}
		return true
	})
	return src, found
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

// toSet returns the deduplicated string slice as a lookup set.
func toSet(in []string) map[string]bool {
	m := make(map[string]bool, len(in))
	for _, s := range in {
		m[s] = true
	}
	return m
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
