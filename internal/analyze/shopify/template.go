package shopify

import (
	"net/url"
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
	TemplateUtility    Template = "utility"
	TemplateUnknown    Template = "unknown"
)

// Classify returns the Shopify template a URL addresses. Later tasks (variant flattening,
// duplicate-path detection, faceted-URL checks) all key off this, so the mapping from path
// shape to Template lives in exactly one place.
func Classify(rawURL string) Template {
	u, err := url.Parse(rawURL)
	if err != nil {
		return TemplateUnknown
	}
	segs := pathSegments(u.Path)
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
	case "search", "cart", "account", "challenge", "checkouts", "orders":
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
			Code:    "shopify-template-schema-gap",
			Message: "Shopify " + string(key.template) + " pages have no " + key.label + " structured data",
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
