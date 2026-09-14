package shopify

import (
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// facetParams are the query parameters Shopify's own collection filtering and sorting use.
// Each one multiplies a single collection into an unbounded set of near-identical URLs. Order
// matters: facetParam walks this slice in order rather than the URL's query map, so the
// parameter it reports is deterministic even when a URL carries more than one of these at
// once (see facetParam).
var facetParams = []string{"sort_by", "filter.", "constraint", "pf_", "grid_list"}

// indexable reports whether a page is open to indexing — no noindex in either the meta robots
// tag or the X-Robots-Tag header.
func indexable(p *crawler.Page) bool {
	if strings.Contains(strings.ToLower(p.Header.Get("X-Robots-Tag")), "noindex") {
		return false
	}
	if p.Doc == nil {
		return true
	}
	robots, _ := p.Doc.Find(`meta[name="robots"]`).First().Attr("content")
	return !strings.Contains(strings.ToLower(robots), "noindex")
}

// canonicalOf returns the page's declared canonical URL, or "" when it has none.
func canonicalOf(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	href, _ := doc.Find(`link[rel="canonical"]`).First().Attr("href")
	return strings.TrimSpace(href)
}

// seoIssues reports the crawlable URLs Shopify generates by default that a store rarely wants
// in an index.
func seoIssues(p *crawler.Page, tmpl Template) []analyze.Issue {
	var issues []analyze.Issue

	// Utility pages carry no content worth ranking, and /search in particular generates an
	// unbounded set of URLs from whatever anyone links to. TemplatePolicy (refund, privacy,
	// terms) is deliberately a separate template from TemplateUtility precisely so it is
	// excluded here — those pages are meant to be indexed.
	if tmpl == TemplateUtility && indexable(p) {
		issues = append(issues, analyze.Issue{
			Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Warning,
			Code:    "shopify-indexable-utility",
			Message: "A Shopify utility page is crawlable and open to indexing",
			Data:    map[string]any{"path": pathOf(p.FinalURL)},
		})
	}

	// A sorted or filtered collection is the same set of products in a different order. Left
	// self-canonical, each permutation competes with the collection it came from.
	if tmpl == TemplateCollection {
		if param, ok := facetParam(p.FinalURL); ok {
			canonical := canonicalOf(p.Doc)
			if canonical == "" || sameURL(canonical, p.FinalURL) {
				issues = append(issues, analyze.Issue{
					Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Warning,
					Code:    "shopify-indexable-facet",
					Message: "A sorted or filtered collection URL is indexable and not canonicalised to the unfiltered collection",
					Data:    map[string]any{"parameter": param, "canonical": canonical},
				})
			}
		}
	}

	// Shopify serves every product at /products/<handle> and again under each collection it
	// belongs to. The nested copies are the same page; without a canonical pointing at the
	// short path, a product with ten collections is ten competing URLs.
	if want, ok := canonicalProductURL(p.FinalURL); ok {
		if canonical := canonicalOf(p.Doc); canonical == "" || !sameURL(canonical, want) {
			issues = append(issues, analyze.Issue{
				Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Warning,
				Code:    "shopify-duplicate-product-path",
				Message: "A product is served under a collection path without a canonical pointing at /products/<handle>",
				Data:    map[string]any{"canonical": canonical, "canonical_should_be": want},
			})
		}
	}
	return issues
}

// facetParam returns the first faceting parameter present in a URL's query, checked in
// facetParams' declared order. It must walk facetParams as the outer loop and the URL's query
// as the inner membership check, not the other way around: u.Query() is a map, and Go
// randomizes map iteration order, so ranging over it as the outer loop would make the
// reported parameter (and therefore the finding's Data) nondeterministic between runs on a
// URL that carries more than one faceting parameter (e.g. ?sort_by=price&filter.v.price.gte=10).
// Reports are diffed between crawls, so that nondeterminism would show up as a spurious diff
// on an unchanged page.
func facetParam(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	q := u.Query()
	for _, f := range facetParams {
		for key := range q {
			if key == f || strings.HasPrefix(key, f) {
				return f, true
			}
		}
	}
	return "", false
}

// sameURL compares two URLs ignoring a trailing slash and fragment, which differ without
// meaning anything.
func sameURL(a, b string) bool {
	return strings.TrimRight(stripFragment(a), "/") == strings.TrimRight(stripFragment(b), "/")
}

// stripFragment drops a URL's fragment.
func stripFragment(raw string) string {
	if i := strings.IndexByte(raw, '#'); i >= 0 {
		return raw[:i]
	}
	return raw
}

// canonicalProductURL returns the /products/<handle> form of a nested
// /collections/<c>/products/<handle> URL. It returns false for any other URL, including the
// canonical product path itself.
//
// A Shopify Markets storefront serves every route under a locale prefix, so the match skips
// that prefix via localeOffset — but the returned URL PUTS IT BACK. Recommending a canonical
// that drops the locale would point the store at a path that does not exist in that market.
func canonicalProductURL(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	segs := pathSegments(u.Path)
	off := localeOffset(segs)
	rest := segs[off:]
	if len(rest) < 4 || rest[0] != "collections" || rest[2] != "products" {
		return "", false
	}
	prefix := ""
	if off > 0 {
		prefix = "/" + segs[0]
	}
	return u.Scheme + "://" + u.Host + prefix + "/products/" + rest[3], true
}

// pathOf returns a URL's path for use in a finding's data.
func pathOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Path
}
