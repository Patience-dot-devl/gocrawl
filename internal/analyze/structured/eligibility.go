package structured

import (
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// fieldSpec splits a type's properties into the three tiers that matter to a search engine.
// A field name may be an any-of group written with "|" separators ("gtin|mpn"), satisfied by
// any one alternative — schema.org offers several interchangeable spellings for the same
// requirement and demanding a specific one would be wrong.
type fieldSpec struct {
	// required fields block rich-result eligibility outright when absent.
	required []string
	// recommended fields do not block eligibility but degrade the result when absent.
	recommended []string
	// merchant fields feed Google's Shopping and free-listing surfaces.
	merchant []string
}

// eligibility is a curated subset of schema.org tied to documented rich-result requirements,
// not a full vocabulary. Adding a type here is the whole cost of covering a new rich result.
var eligibility = map[string]fieldSpec{
	"Product": {
		required:    []string{"name", "image", "offers.price", "offers.priceCurrency", "offers.availability"},
		recommended: []string{"brand", "sku", "description", "aggregateRating", "review"},
		merchant: []string{
			"gtin|gtin8|gtin12|gtin13|gtin14|mpn",
			"priceValidUntil",
			"offers.shippingDetails",
			"hasMerchantReturnPolicy",
		},
	},
	"ProductGroup": {
		required:    []string{"name"},
		recommended: []string{"hasVariant", "productGroupID", "variesBy", "image", "brand"},
		// Google accepts these merchant fields on the ProductGroup itself or on each
		// variant's Offer. A store that models variants correctly (ProductGroup +
		// hasVariant, exactly what shopify-flat-variant-product tells owners to adopt)
		// has no top-level Product to carry them: hasVariant children are exempt as
		// thin copies (see listProperties below), so without this entry the merchant
		// tier never runs on the best-structured stores.
		merchant: []string{
			"gtin|gtin8|gtin12|gtin13|gtin14|mpn",
			"priceValidUntil",
			"offers.shippingDetails",
			"hasMerchantReturnPolicy",
		},
	},
	"Article": {
		required:    []string{"headline"},
		recommended: []string{"image", "datePublished", "dateModified", "author.name", "publisher.name"},
	},
	"NewsArticle": {
		required:    []string{"headline"},
		recommended: []string{"image", "datePublished", "dateModified", "author.name", "publisher.name"},
	},
	"BlogPosting": {
		required:    []string{"headline"},
		recommended: []string{"image", "datePublished", "dateModified", "author.name", "publisher.name"},
	},
	"Recipe": {
		required:    []string{"name"},
		recommended: []string{"image", "recipeIngredient", "recipeInstructions", "author.name", "totalTime"},
	},
	"Event": {
		required:    []string{"name", "startDate"},
		recommended: []string{"location", "image", "endDate", "eventStatus", "offers.url"},
	},
	"Organization": {
		required:    []string{"name"},
		recommended: []string{"url", "logo", "sameAs"},
	},
	"LocalBusiness": {
		required:    []string{"name"},
		recommended: []string{"address", "telephone", "openingHours|openingHoursSpecification", "geo", "priceRange"},
	},
	"Person": {
		required:    []string{"name"},
		recommended: []string{"url", "sameAs", "jobTitle"},
	},
	"BreadcrumbList": {
		required: []string{"itemListElement"},
	},
	"FAQPage": {
		required: []string{"mainEntity"},
	},
	"ItemList": {
		required: []string{"itemListElement"},
	},
	"VideoObject": {
		required:    []string{"name", "thumbnailUrl"},
		recommended: []string{"description", "uploadDate", "duration", "contentUrl|embedUrl"},
	},
	"WebSite": {
		required:    []string{"name"},
		recommended: []string{"url", "potentialAction"},
	},
}

// listProperties name the places schema.org puts thin, deliberately incomplete copies of an
// entity: a collection page's product tiles, a ProductGroup's variants, a "customers also
// bought" rail. Those copies are supposed to be minimal, so eligibility checking them would
// put a warning on every tile of every listing page and say nothing true.
var listProperties = []string{
	".itemListElement", ".hasVariant", ".isVariantOf",
	".isSimilarTo", ".isRelatedTo", ".isAccessoryOrSparePartFor",
}

// exemptFromEligibility reports whether a node's path puts it inside one of listProperties.
func exemptFromEligibility(path string) bool {
	for _, prop := range listProperties {
		if strings.Contains(path, prop) {
			return true
		}
	}
	return false
}

// requiredIssues checks every eligible node against its type's field tiers. Required gaps are
// returned as per-page issues; recommended and merchant gaps go into the rollup, which the
// caller renders once per crawl.
func requiredIssues(p *crawler.Page, g schemaorg.Graph, roll *rollup) []analyze.Issue {
	var issues []analyze.Issue
	for _, n := range g.Nodes {
		if exemptFromEligibility(n.Path) {
			continue
		}
		for _, ty := range n.Types {
			spec, known := eligibility[ty]
			if !known {
				continue
			}
			if missing := missingFields(g, n, spec.required); len(missing) > 0 {
				issues = append(issues, analyze.Issue{
					Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
					Code:    "structured-missing-required",
					Message: "Structured-data object is missing required schema.org fields",
					Data:    map[string]any{"type": ty, "missing": missing, "path": n.Path},
				})
			}
			roll.add("structured-missing-recommended", ty, missingFields(g, n, spec.recommended), p.FinalURL)
			roll.add("structured-missing-merchant", ty, missingFields(g, n, spec.merchant), p.FinalURL)
		}
	}
	return issues
}

// missingFields returns the entries of want that n does not satisfy. An entry containing "|"
// is an any-of group, satisfied by any one alternative, and is reported by its full group
// name so a reader sees the choice rather than an arbitrary member of it.
func missingFields(g schemaorg.Graph, n schemaorg.Node, want []string) []string {
	var missing []string
	for _, field := range want {
		satisfied := false
		for _, alt := range strings.Split(field, "|") {
			if g.HasValue(n, alt) {
				satisfied = true
				break
			}
		}
		if !satisfied {
			missing = append(missing, field)
		}
	}
	return missing
}
