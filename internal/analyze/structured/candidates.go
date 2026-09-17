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

// ctaSelector matches the interactive elements a shopper can actually activate. Matching
// cartSignalRe against whole-page body text (the old approach) lets prose stand in for a
// control — a free-shipping strip reading "add it to your cart to qualify" is a sentence,
// not a button — so the signal is restricted to an element a click/tap can land on.
const ctaSelector = `button, input[type="submit"], a, [role="button"]`

// maxCTAAncestorHops bounds how far hasProductSignal climbs from a cart/buy control that has
// no enclosing <form> while looking for a co-located price. Real themes nest the control a
// few wrapper divs deep from the price it belongs to (e.g. .product-form__buy-buttons >
// .price), so a handful of hops covers that; climbing further starts pulling in unrelated
// regions of the page.
const maxCTAAncestorHops = 4

// maxProductCTAPairs bounds how many co-located CTA/price pairs a single product is expected
// to produce. A product page can legitimately duplicate its buy control (a sticky buy-bar
// mirroring the main product form), so two pairs is normal; a listing/collection page with
// several quick-add tiles produces many more. At or under the bound, treat the page as one
// product; over it, treat it as a listing and stay silent — this is what separates a real
// product page from a collection page without ever reading a schema.org type off the page
// (see the Allbirds product page below, which relies on exactly that).
const maxProductCTAPairs = 2

// ctaLandmarkTags are ancestor elements hasProductSignal refuses to use as a fallback price
// container for a CTA with no enclosing <form>. They are page-level layout regions that
// legitimately span unrelated content, not a single control's own container — reaching one
// while climbing means give up, not "search everything below it". This is precisely the
// shape of the false positive this heuristic used to produce: a sitewide free-shipping strip
// ("Free shipping over $35") in the header and a persistent mini-cart drawer's "Add to cart"
// button are both just a couple of hops under <body>, so treating <body> (or another
// landmark) as fair game re-creates the whole-page match this rewrite exists to kill.
var ctaLandmarkTags = map[string]bool{
	"html": true, "body": true, "header": true, "footer": true, "nav": true, "main": true,
}

// videoHostRe matches iframe embeds from the video platforms most pages use instead of a
// native <video> element.
var videoHostRe = regexp.MustCompile(`(?i)youtube(-nocookie)?\.com|vimeo\.com`)

// candidateIssues runs low-noise heuristics that flag content shaped like a common schema.org
// type (breadcrumbs, a product, an article, an embedded video) whose matching JSON-LD is
// absent from types. Each check requires a reasonably specific on-page signal so the
// suggestion stays actionable rather than firing on every page.
//
// The breadcrumb check is the exception to per-page reporting: a breadcrumb trail is template
// chrome, so a theme without BreadcrumbList lacks it on every page. It feeds roll and surfaces
// as one site-wide finding. The product, article and video checks depend on page content and
// stay per page.
func candidateIssues(p *crawler.Page, g schemaorg.Graph, roll *rollup) []analyze.Issue {
	doc := p.Doc
	var issues []analyze.Issue
	add := func(code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{
			Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning, Code: code, Message: msg, Data: data,
		})
	}

	if !g.HasType("BreadcrumbList") {
		if links, ok := hasBreadcrumbMarkup(doc); ok {
			roll.addBreadcrumb(p.FinalURL, analyze.CanonicalURL(p), links)
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
// Product/price microdata, or a price co-located with a cart/buy call-to-action.
//
// "Co-located" is load-bearing. Matching cartSignalRe against the whole page and priceRe
// against the whole page independently — the original implementation — is satisfied by two
// pieces of unrelated sitewide boilerplate: a free-shipping-threshold strip ("Free shipping
// over $35") supplies the price, and a persistent mini-cart/quick-add drawer supplies "Add to
// cart", on every single page of a real Shopify store (homepage, policy pages, blog articles
// included). The two signals have to come from the same control: the price must sit inside
// the CTA's own <form>, or failing that a bounded ancestor (see ctaPriceContainer).
//
// A collection/listing page defeats even that: it has many genuine CTA/price pairs (one per
// tile) where a product page has one (or two, for a duplicated sticky buy-bar). Counting
// pairs — not reading @type off the page — is what tells a collection page apart from the
// Allbirds product page that is this feature's motivating example precisely because it
// declares CollectionPage+FAQPage and no Product.
func hasProductSignal(doc *goquery.Document) (string, bool) {
	if doc.Find(`[itemprop="price"], [itemtype*="schema.org/Product"], [itemtype*="schema.org/Offer"]`).Length() > 0 {
		return "microdata", true
	}

	var prices []string
	doc.Find(ctaSelector).Each(func(_ int, cta *goquery.Selection) {
		// A control nested inside an aria-hidden/hidden ancestor (a closed Shopify cart
		// drawer, typically) was never actually shown to a visitor, so its label and any
		// price near it can't stand in for something a shopper saw. visibleText only
		// strips hidden *descendants* of the container it's called on; a hidden ancestor
		// of the CTA itself has to be checked separately, here.
		if cta.Closest(`[aria-hidden="true"], [hidden]`).Length() > 0 {
			return
		}
		if !cartSignalRe.MatchString(ctaLabel(cta)) {
			return
		}
		container, ok := ctaPriceContainer(cta)
		if !ok {
			return
		}
		if price := priceRe.FindString(visibleText(container)); price != "" {
			prices = append(prices, price)
		}
	})

	if len(prices) == 0 || len(prices) > maxProductCTAPairs {
		return "", false
	}
	return prices[0], true
}

// ctaLabel returns the visible label of a CTA candidate: an <input type="submit">'s value
// (it has no text children), otherwise the element's own visible text.
func ctaLabel(cta *goquery.Selection) string {
	if goquery.NodeName(cta) == "input" {
		val, _ := cta.Attr("value")
		return val
	}
	return visibleText(cta)
}

// ctaPriceContainer returns the element hasProductSignal should search for a co-located
// price: the CTA's nearest enclosing <form> if it has one, otherwise the nearest ancestor
// within maxCTAAncestorHops that isn't a page-level landmark (see ctaLandmarkTags). A
// control with no enclosing form and no qualifying ancestor within the hop bound reports
// not-found rather than falling back to an unbounded search.
func ctaPriceContainer(cta *goquery.Selection) (*goquery.Selection, bool) {
	if form := cta.Closest("form"); form.Length() > 0 {
		return form, true
	}
	anc := cta.Parent()
	for hop := 0; hop < maxCTAAncestorHops && anc.Length() > 0; hop++ {
		if ctaLandmarkTags[goquery.NodeName(anc)] {
			return nil, false
		}
		if priceRe.MatchString(visibleText(anc)) {
			return anc, true
		}
		anc = anc.Parent()
	}
	return nil, false
}

// visibleText returns s's text with <script>/<template> contents and aria-hidden/hidden
// subtrees stripped. goquery's Text() walks every descendant text node with no tag
// exclusion, so without this a JSON config blob or an off-screen, aria-hidden="true" cart
// drawer — exactly how Shopify themes mark up the drawer before it's opened — could supply
// a price or CTA text that was never actually rendered to the shopper.
func visibleText(s *goquery.Selection) string {
	clone := s.Clone()
	clone.Find(`script, template, [aria-hidden="true"], [hidden]`).Remove()
	return clone.Text()
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
