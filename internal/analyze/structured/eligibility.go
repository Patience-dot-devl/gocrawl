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
		// Google's product-variants documentation requires only name on the group;
		// hasVariant and productGroupID are recommended (a group may instead be joined by
		// each variant's isVariantOf/inProductGroupWithID). image is not a documented group
		// property at all — variant images belong on each variant (see variantIssues).
		required:    []string{"name"},
		recommended: []string{"hasVariant", "productGroupID", "variesBy", "brand", "description", "aggregateRating", "review"},
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
// put a warning on every tile of every listing page and say nothing true. A ProductGroup's
// variants are the one exception that is still checked, against their own variant rules and
// rolled up rather than per node — see variantIssues.
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
	canonical := analyze.CanonicalURL(p)
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
			roll.add("structured-missing-recommended", ty, missingFields(g, n, spec.recommended, nil), p.FinalURL, canonical)
			roll.add("structured-missing-merchant", ty, missingFields(g, n, spec.merchant, merchantFallback(ty, g, n)), p.FinalURL, canonical)
			if ty == "ProductGroup" {
				roll.add("structured-variant-incomplete", ty, variantGaps(g, n), p.FinalURL, canonical)
			}
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

// variantRequired is what Google requires on each variant declared inline under a
// ProductGroup's hasVariant: the standard Product fields plus a unique identifier. The group
// can carry brand, description and ratings once for all variants, but not these.
var variantRequired = []string{
	"name", "image", "offers.price", "offers.priceCurrency",
	"sku|gtin|gtin8|gtin12|gtin13|gtin14",
}

// variesByProperties maps the variesBy values Google supports to the property each variant
// must then carry. Anything else in variesBy is not a documented variant dimension and is
// ignored rather than guessed at.
var variesByProperties = map[string]bool{
	"color": true, "size": true, "suggestedAge": true,
	"suggestedGender": true, "material": true, "pattern": true,
}

// stubKeys are the keys a variant reference may carry and still be a reference. Google's own
// multi-page example lists the variants served on other pages as {"url": ...}; their full
// markup lives on those pages, so checking the stub here would report every variant of every
// multi-page store as incomplete.
var stubKeys = map[string]bool{"@type": true, "@id": true, "@context": true, "url": true}

// variantGaps returns the union of fields missing across n's inline variants, each listed once
// however many variants lack it. The rollup counts pages, not variants: a product with forty
// sizes missing gtin is one template fact, not forty findings.
func variantGaps(g schemaorg.Graph, n schemaorg.Node) []string {
	want := append(append([]string(nil), variantRequired...), variesBy(g, n)...)
	seen := make(map[string]bool)
	var out []string
	for _, v := range g.NodesAt(n, "hasVariant") {
		if isStub(v) {
			continue
		}
		for _, f := range missingFields(g, v, want, nil) {
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// variesBy returns the supported variant dimensions a ProductGroup declares, accepting both
// the full "https://schema.org/color" form Google documents and a bare "color".
func variesBy(g schemaorg.Graph, n schemaorg.Node) []string {
	var out []string
	for _, raw := range g.Strs(n, "variesBy") {
		name := raw
		if i := strings.LastIndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		if variesByProperties[name] {
			out = append(out, name)
		}
	}
	return out
}

// isStub reports whether a variant is a bare reference to markup served elsewhere.
func isStub(v schemaorg.Node) bool {
	for k := range v.Props {
		if !stubKeys[k] {
			return false
		}
	}
	return true
}
