package shopify

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Template is the kind of page a Shopify URL addresses. Shopify's URL structure is fixed by
// the platform rather than chosen per store, which is what makes classifying by path
// reliable here in a way it would not be on an arbitrary site.
type Template string

const (
	TemplateHome       Template = "home"
	TemplateProduct    Template = "product"
	TemplateCollection Template = "collection"
	TemplateArticle    Template = "article"
	TemplateBlog       Template = "blog"
	TemplatePage       Template = "page"
	// TemplatePolicy is Shopify's built-in legal pages (/policies/refund-policy,
	// /policies/privacy-policy, /policies/terms-of-service, ...). It is kept separate from
	// TemplateUtility deliberately: TemplateUtility means "should not be indexable" (Task 13
	// flags indexable utility pages), but policy pages are meant to be indexed, so filing them
	// under TemplateUtility would make Task 13 false-positive on every store's policy pages.
	TemplatePolicy  Template = "policy"
	TemplateUtility Template = "utility"
	TemplateUnknown Template = "unknown"
)

// localeSegRe matches a Shopify Markets locale or locale-region prefix, e.g. "fr" or "en-ca".
var localeSegRe = regexp.MustCompile(`^[a-z]{2}(-[a-z]{2})?$`)

// localeRootSegments are the first-segment names Shopify's own routes use. None of them is
// two letters today, so this exclusion is not load-bearing against any current root — it is
// kept anyway as cheap insurance against a two-letter app-proxy path such as /tr/... being
// mistaken for a locale prefix.
var localeRootSegments = map[string]bool{
	"products": true, "collections": true, "blogs": true, "pages": true,
	"cart": true, "search": true, "account": true, "challenge": true,
	"checkouts": true, "orders": true, "policies": true,
}

// localeOffset returns the index of the first path segment that is part of Shopify's own
// route structure, skipping a leading Shopify Markets locale prefix such as "en-ca" or "fr".
// A Markets storefront serves every route under that prefix, so classifying on segs[0] would
// put an entire localized store in the unknown bucket and silence every check below.
// The locale is only skipped for classification — callers that rebuild a URL must keep it,
// or they will point a canonical at a path that does not exist in that market.
func localeOffset(segs []string) int {
	if len(segs) == 0 {
		return 0
	}
	if localeSegRe.MatchString(segs[0]) && !localeRootSegments[segs[0]] {
		return 1
	}
	return 0
}

// Classify returns the Shopify template a URL addresses. Later tasks (variant flattening,
// duplicate-path detection, faceted-URL checks) all key off this, so the mapping from path
// shape to Template lives in exactly one place.
func Classify(rawURL string) Template {
	u, err := url.Parse(rawURL)
	if err != nil {
		return TemplateUnknown
	}
	segs := pathSegments(u.Path)
	segs = segs[localeOffset(segs):]
	if len(segs) == 0 {
		return TemplateHome
	}
	switch segs[0] {
	case "products":
		if len(segs) >= 2 {
			return TemplateProduct
		}
	case "collections":
		// /collections/<handle>/products/<handle> is the same product reached through a
		// collection; Shopify serves it from the product template.
		if len(segs) >= 4 && segs[2] == "products" {
			return TemplateProduct
		}
		if len(segs) >= 2 {
			return TemplateCollection
		}
	case "blogs":
		// /blogs/<handle>/tagged/<tag> is a filtered listing of posts, not a single post —
		// it belongs with the blog template, not the article template.
		if len(segs) >= 3 && segs[2] == "tagged" {
			return TemplateBlog
		}
		if len(segs) >= 3 {
			return TemplateArticle
		}
		if len(segs) == 2 {
			return TemplateBlog
		}
	case "pages":
		if len(segs) >= 2 {
			return TemplatePage
		}
	case "policies":
		if len(segs) >= 2 {
			return TemplatePolicy
		}
	case "search", "cart", "account", "challenge", "checkouts", "orders", "password":
		return TemplateUtility
	}
	return TemplateUnknown
}

// pathSegments splits a URL path into its non-empty segments.
func pathSegments(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// expectation is one schema.org requirement for a template, satisfied by any one of anyOf.
// The alternatives exist because more than one type legitimately answers the same need — a
// collection listing is equally well described by CollectionPage or ItemList.
type expectation struct {
	template Template
	anyOf    []string
	label    string
}

// expectations is the schema each Shopify template should carry. Utility templates are absent
// deliberately: a cart or an account page has nothing to say to a search engine.
var expectations = []expectation{
	{TemplateHome, []string{"Organization", "LocalBusiness"}, "Organization"},
	{TemplateHome, []string{"WebSite"}, "WebSite"},
	{TemplateProduct, []string{"Product", "ProductGroup"}, "Product"},
	{TemplateProduct, []string{"BreadcrumbList"}, "BreadcrumbList"},
	{TemplateCollection, []string{"CollectionPage", "ItemList"}, "CollectionPage or ItemList"},
	{TemplateCollection, []string{"BreadcrumbList"}, "BreadcrumbList"},
	{TemplateArticle, []string{"BlogPosting", "Article", "NewsArticle"}, "BlogPosting"},
	{TemplateArticle, []string{"BreadcrumbList"}, "BreadcrumbList"},
	{TemplateBlog, []string{"Blog", "CollectionPage"}, "Blog"},
	{TemplatePage, []string{"WebPage", "AboutPage", "ContactPage", "FAQPage"}, "WebPage"},
}

// gapKey identifies one accumulated template gap.
type gapKey struct {
	template Template
	label    string
}

// gapEntry counts the pages of a template missing one expectation.
type gapEntry struct {
	anyOf    []string
	pages    int
	examples []string
}

// maxExamples caps the example URLs on an aggregated finding, matching the structured
// analyzer's rollup.
const maxExamples = 5

// templateGapIssues reports, per template, the schema every page of that template is missing.
// Shopify templates are shared across every page they render, so a gap is a property of the
// template — reporting it per page would repeat one fact several hundred times.
func templateGapIssues(result *crawler.Result, base string) []analyze.Issue {
	entries := make(map[gapKey]*gapEntry)
	var order []gapKey

	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		tmpl := Classify(p.FinalURL)
		g, _ := schemaorg.Parse(p.Doc)
		for _, exp := range expectations {
			if exp.template != tmpl || g.HasType(exp.anyOf...) {
				continue
			}
			key := gapKey{template: tmpl, label: exp.label}
			e, ok := entries[key]
			if !ok {
				e = &gapEntry{anyOf: exp.anyOf}
				entries[key] = e
				order = append(order, key)
			}
			e.pages++
			if len(e.examples) < maxExamples {
				e.examples = append(e.examples, p.FinalURL)
			}
		}
	}

	sort.Slice(order, func(i, j int) bool {
		if order[i].template != order[j].template {
			return order[i].template < order[j].template
		}
		return order[i].label < order[j].label
	})

	var issues []analyze.Issue
	for _, key := range order {
		e := entries[key]
		issues = append(issues, analyze.Issue{
			Analyzer: "shopify", URL: base, Severity: analyze.Warning,
			Code: "shopify-template-schema-gap",
			Message: fmt.Sprintf("%d Shopify %s %s no %s structured data",
				e.pages, key.template, pageCountVerb(e.pages), key.label),
			Data: map[string]any{
				"template": string(key.template),
				"expected": e.anyOf,
				"pages":    e.pages,
				"examples": e.examples,
			},
		})
	}
	return issues
}

// pageCountVerb returns the subject-verb agreement for a page count, so a single-page gap
// reads as "1 ... page has ..." rather than the grammatically wrong "1 ... pages have ...".
func pageCountVerb(pages int) string {
	if pages == 1 {
		return "page has"
	}
	return "pages have"
}
