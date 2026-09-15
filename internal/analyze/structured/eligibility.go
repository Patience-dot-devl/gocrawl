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
		required: []string{"name", "image", "offers.price", "offers.priceCurrency"},
		// offers.availability is Google-documented as recommended, not blocking: an Offer
		// missing it is still eligible for the product rich result, just a weaker listing.
		recommended: []string{"offers.availability", "brand", "sku", "description", "aggregateRating", "review"},
		// priceValidUntil and hasMerchantReturnPolicy are documented by Google on offers,
		// not on Product itself; either placement satisfies the check (integrity.go's
		// dateProperties already accepts both spellings for the date one, at :195-196).
		merchant: []string{
			"gtin|gtin8|gtin12|gtin13|gtin14|mpn",
			"priceValidUntil|offers.priceValidUntil",
			"offers.shippingDetails",
			"hasMerchantReturnPolicy|offers.hasMerchantReturnPolicy",
			"offers.itemCondition",
		},
	},
	"ProductGroup": {
		// hasVariant and productGroupID are both required by Google's product-variants
		// documentation: productGroupID is what joins the variants into one group, and a
		// ProductGroup with no hasVariant array has nothing to group. hasVariant children
		// remain exempt from their own required-field check as thin copies (see
		// listProperties below), so this only checks that the group node names them.
		required:    []string{"name", "hasVariant", "productGroupID"},
		recommended: []string{"variesBy", "image", "brand"},
		// Google accepts these merchant fields on the ProductGroup itself or on each
		// variant's Offer. A store that models variants correctly (ProductGroup +
		// hasVariant, exactly what shopify-flat-variant-product tells owners to adopt)
		// has no top-level Product to carry them: hasVariant children are exempt as
		// thin copies (see listProperties below), so without this entry the merchant
		// tier never runs on the best-structured stores. missingFields falls through to
		// the variants for this type (see satisfiedOnVariants) so a store that puts these
		// fields on each variant's Offer, rather than the group, is not told it is
		// missing everything.
		merchant: []string{
			"gtin|gtin8|gtin12|gtin13|gtin14|mpn",
			"priceValidUntil|offers.priceValidUntil",
			"offers.shippingDetails",
			"hasMerchantReturnPolicy|offers.hasMerchantReturnPolicy",
			"offers.itemCondition",
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
		// image is required; recipeIngredient/recipeInstructions are genuinely only
		// recommended per Google's recipe documentation, despite how central they feel.
		required:    []string{"name", "image"},
		recommended: []string{"recipeIngredient", "recipeInstructions", "author.name", "totalTime"},
	},
	"Event": {
		// location is required for every Event, including online ones: a VirtualLocation
		// node with a url satisfies it.
		required:    []string{"name", "startDate", "location"},
		recommended: []string{"image", "endDate", "eventStatus", "offers.url"},
	},
	"Organization": {
		// url and logo are both required by Google's logo guidance, not merely recommended.
		required:    []string{"name", "logo", "url"},
		recommended: []string{"sameAs"},
	},
	"LocalBusiness": {
		required:    []string{"name", "address"},
		recommended: []string{"telephone", "openingHours|openingHoursSpecification", "geo", "priceRange"},
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
		// Google's required set for the video rich result is name + description +
		// thumbnailUrl + uploadDate; without any one of the four there is no video result
		// at all, so all four belong here, not split across tiers.
		required:    []string{"name", "thumbnailUrl", "description", "uploadDate"},
		recommended: []string{"duration", "contentUrl|embedUrl"},
	},
	"WebSite": {
		// potentialAction (Sitelinks Search Box markup) is deliberately absent: Google
		// deprecated the Sitelinks Search Box feature in November 2023, so recommending it
		// is stale advice that would send readers to build markup for a feature that no
		// longer renders. Do not re-add it without checking whether Google has reversed
		// that deprecation.
		required: []string{"name", "url"},
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
			if missing := missingFields(g, n, spec.required, nil); len(missing) > 0 {
				issues = append(issues, analyze.Issue{
					Analyzer: "structured", URL: p.FinalURL, Severity: analyze.Warning,
					Code:    "structured-missing-required",
					Message: "Structured-data object is missing required schema.org fields",
					Data:    map[string]any{"type": ty, "missing": missing, "path": n.Path},
				})
			}
			roll.add("structured-missing-recommended", ty, missingFields(g, n, spec.recommended, nil), p.FinalURL)
			roll.add("structured-missing-merchant", ty, missingFields(g, n, spec.merchant, merchantFallback(ty, g, n)), p.FinalURL)
		}
	}
	return issues
}

// merchantFallback returns the variant fallback for the merchant tier, or nil for every other
// type. Only ProductGroup has variants to fall through to (a plain Product has none), and only
// the merchant tier accepts the field on a variant's Offer as an alternative to the group's own
// — required and recommended fields must still be satisfied by the node itself.
func merchantFallback(ty string, g schemaorg.Graph, n schemaorg.Node) func(string) bool {
	if ty != "ProductGroup" {
		return nil
	}
	return func(field string) bool { return satisfiedOnVariants(g, n, field) }
}

// satisfiedOnVariants reports whether field (or, for an any-of group, any alternative in it)
// resolves under n's hasVariant array. Google accepts the ProductGroup merchant fields on the
// group itself or on each variant's Offer; g.HasValue's existing array-walking machinery
// already resolves a path like "hasVariant.gtin13" or "hasVariant.offers.shippingDetails"
// through the variant array, so this only has to prefix the path and reuse it.
func satisfiedOnVariants(g schemaorg.Graph, n schemaorg.Node, field string) bool {
	for _, alt := range strings.Split(field, "|") {
		if g.HasValue(n, "hasVariant."+alt) {
			return true
		}
	}
	return false
}

// missingFields returns the entries of want that n does not satisfy. An entry containing "|"
// is an any-of group, satisfied by any one alternative, and is reported by its full group name
// so a reader sees the choice rather than an arbitrary member of it — even when satisfaction
// came from fallback, since widening the group string itself to include a "hasVariant.…"
// alternative would make the printed label unreadable (see satisfiedOnVariants). fallback is
// nil for every tier except merchant-on-ProductGroup; see merchantFallback.
func missingFields(g schemaorg.Graph, n schemaorg.Node, want []string, fallback func(field string) bool) []string {
	var missing []string
	for _, field := range want {
		satisfied := false
		for _, alt := range strings.Split(field, "|") {
			if g.HasValue(n, alt) {
				satisfied = true
				break
			}
		}
		if !satisfied && fallback != nil {
			satisfied = fallback(field)
		}
		if !satisfied {
			missing = append(missing, field)
		}
	}
	return missing
}
