package structured

import (
	"regexp"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

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
func candidateIssues(p *crawler.Page, g schemaorg.Graph) []analyze.Issue {
	doc := p.Doc
	var issues []analyze.Issue
	add := func(code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning, Code: code, Message: msg, Data: data,
		})
	}

	if !g.HasType("BreadcrumbList") {
		if links, ok := hasBreadcrumbMarkup(doc); ok {
			add("structured-breadcrumb-candidate", "Page has breadcrumb navigation but no BreadcrumbList structured data",
				map[string]any{"links": links})
		}
	}

	if !g.HasType("Product", "Offer") {
		if signal, ok := hasProductSignal(doc); ok {
			add("structured-product-candidate", "Page reads like a product page but has no Product structured data",
				map[string]any{"signal": signal})
		}
	}

	hasArticleType := g.HasType("Article", "NewsArticle", "BlogPosting", "TechArticle", "Report")
	if !hasArticleType {
		if words, ok := hasArticleSignal(doc); ok {
			add("structured-article-candidate", "Page reads like an article but has no Article/NewsArticle/BlogPosting structured data",
				map[string]any{"words": words})
		}
	}

	if !g.HasType("VideoObject") {
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
