package structured_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/structured"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// pages builds a Result from several (url, html) pairs so rollup behaviour can be asserted
// across a crawl rather than a single page.
func pages(t *testing.T, seed string, urlHTML map[string]string) *crawler.Result {
	t.Helper()
	res := &crawler.Result{Seed: seed}
	for u, html := range urlHTML {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatalf("parse %s: %v", u, err)
		}
		res.Pages = append(res.Pages, &crawler.Page{
			FinalURL: u, StatusCode: 200, ContentType: "text/html", Doc: doc,
		})
	}
	return res
}

func findAll(issues []analyze.Issue, code string) []analyze.Issue {
	var out []analyze.Issue
	for _, is := range issues {
		if is.Code == code {
			out = append(out, is)
		}
	}
	return out
}

// completeProduct carries every required Product field and nothing else, so a test can add
// exactly one tier's worth of fields and assert which code fires.
const completeProduct = `{"@context":"https://schema.org","@type":"Product","name":"Tee",
 "image":"https://shop.test/t.jpg",
 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}`

func TestProductMissingRequiredFieldIsPerPage(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required")
	if !ok {
		t.Fatal("expected structured-missing-required for a Product with no offers")
	}
	// offers.availability is Google-documented as recommended, not required (see the tier
	// correction in eligibility.go), so a Product missing only price/priceCurrency/availability
	// is missing two required fields, not three.
	missing, _ := is.Data["missing"].([]string)
	want := map[string]bool{"offers.price": true, "offers.priceCurrency": true}
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing offer fields, got %v", missing)
	}
	for _, m := range missing {
		if !want[m] {
			t.Errorf("unexpected missing field %q", m)
		}
	}
	if is.URL != "https://example.com/" {
		t.Errorf("expected a per-page URL, got %q", is.URL)
	}
}

func TestCompleteProductRaisesNoRequiredGap(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">`+completeProduct+`</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Error("a Product with all four required fields should raise no required gap")
	}
}

// TestProductMissingOnlyAvailabilityRaisesNoRequiredGap proves the tier correction: Google
// documents offers.availability as recommended, not required, so an Offer missing only
// availability must not trigger structured-missing-required.
func TestProductMissingOnlyAvailabilityRaisesNoRequiredGap(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD"}}
	</script></head><body></body></html>`)
	if is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Errorf("a Product missing only offers.availability must not raise structured-missing-required, got %+v", is.Data)
	}
}

func TestRecommendedGapRollsUpAcrossPages(t *testing.T) {
	html := `<html><head><script type="application/ld+json">` + completeProduct + `</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{
		"https://shop.test/products/a": html,
		"https://shop.test/products/b": html,
		"https://shop.test/products/c": html,
	})
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-missing-recommended")
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 rolled-up issue for 3 pages, got %d", len(got))
	}
	is := got[0]
	if is.Severity != analyze.Info {
		t.Errorf("expected info severity, got %q", is.Severity)
	}
	if is.URL != "https://shop.test" {
		t.Errorf("expected the site base as the issue URL, got %q", is.URL)
	}
	if is.Data["type"] != "Product" {
		t.Errorf("expected type Product, got %v", is.Data["type"])
	}
	// The rollup emits one issue per type at the same code and URL; the instance is what keeps
	// them apart in gocrawl compare.
	if is.Data[analyze.InstanceKey] != "Product" {
		t.Errorf("expected instance Product, got %v", is.Data[analyze.InstanceKey])
	}
	if is.Data["pages"] != 3 {
		t.Errorf("expected pages=3, got %v", is.Data["pages"])
	}
	missing, _ := is.Data["missing"].(map[string]int)
	if missing["brand"] != 3 || missing["aggregateRating"] != 3 {
		t.Errorf("expected brand and aggregateRating missing on all 3 pages, got %v", missing)
	}
	examples, _ := is.Data["examples"].([]string)
	if len(examples) != 3 {
		t.Errorf("expected 3 example URLs, got %v", examples)
	}
}

func TestRollupCapsExamplesAtFive(t *testing.T) {
	html := `<html><head><script type="application/ld+json">` + completeProduct + `</script></head><body></body></html>`
	urls := map[string]string{}
	for _, s := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		urls["https://shop.test/products/"+s] = html
	}
	res := pages(t, "https://shop.test", urls)
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-missing-recommended")
	if len(got) != 1 {
		t.Fatalf("expected 1 rolled-up issue, got %d", len(got))
	}
	if got[0].Data["pages"] != 7 {
		t.Errorf("expected pages=7, got %v", got[0].Data["pages"])
	}
	examples, _ := got[0].Data["examples"].([]string)
	if len(examples) != 5 {
		t.Errorf("expected examples capped at 5, got %d", len(examples))
	}
}

func TestMerchantGapUsesItsOwnCode(t *testing.T) {
	html := `<html><head><script type="application/ld+json">` + completeProduct + `</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	issues := structured.New().Analyze(context.Background(), res)
	merchant := findAll(issues, "structured-missing-merchant")
	if len(merchant) != 1 {
		t.Fatalf("expected 1 merchant-gap issue, got %d", len(merchant))
	}
	missing, _ := merchant[0].Data["missing"].(map[string]int)
	for _, f := range []string{"gtin|gtin8|gtin12|gtin13|gtin14|mpn", "priceValidUntil|offers.priceValidUntil", "offers.shippingDetails", "hasMerchantReturnPolicy|offers.hasMerchantReturnPolicy", "offers.itemCondition"} {
		if missing[f] != 1 {
			t.Errorf("expected %q reported missing once, got %d", f, missing[f])
		}
	}
	// A merchant field must never appear under the recommended code.
	for _, is := range findAll(issues, "structured-missing-recommended") {
		m, _ := is.Data["missing"].(map[string]int)
		if _, leaked := m["hasMerchantReturnPolicy"]; leaked {
			t.Error("merchant fields must not leak into the recommended rollup")
		}
	}
}

// TestMerchantGapSeverityIsWarningRecommendedStaysInfo is the F2 regression: the merchant tier
// is the commercial point of this analyzer, so its gap must be loud enough to surface in a
// severity=warning filter, while the recommended tier (genuinely optional polish) stays info.
func TestMerchantGapSeverityIsWarningRecommendedStaysInfo(t *testing.T) {
	html := `<html><head><script type="application/ld+json">` + completeProduct + `</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	issues := structured.New().Analyze(context.Background(), res)

	merchant, ok := find(issues, "structured-missing-merchant")
	if !ok {
		t.Fatal("expected a structured-missing-merchant issue")
	}
	if merchant.Severity != analyze.Warning {
		t.Errorf("expected structured-missing-merchant at warning, got %q", merchant.Severity)
	}

	recommended, ok := find(issues, "structured-missing-recommended")
	if !ok {
		t.Fatal("expected a structured-missing-recommended issue")
	}
	if recommended.Severity != analyze.Info {
		t.Errorf("expected structured-missing-recommended to stay info, got %q", recommended.Severity)
	}
}

func TestAnyOfMerchantFieldSatisfiedByOneAlternative(t *testing.T) {
	html := `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg","mpn":"AC-1",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-missing-merchant")
	if len(got) != 1 {
		t.Fatalf("expected 1 merchant-gap issue, got %d", len(got))
	}
	missing, _ := got[0].Data["missing"].(map[string]int)
	if _, present := missing["gtin|gtin8|gtin12|gtin13|gtin14|mpn"]; present {
		t.Error("mpn alone should satisfy the gtin-or-mpn identifier requirement")
	}
}

// TestProductOffersPlacementSatisfiesMerchantFields is the F1 regression: Google documents
// priceValidUntil and hasMerchantReturnPolicy on offers, not on the Product itself, and the
// merchant tier must accept either placement. Before the fix these bare paths never resolve
// against markup that (correctly) nests them under offers, so a store doing this right was
// told it was missing both fields.
func TestProductOffersPlacementSatisfiesMerchantFields(t *testing.T) {
	html := `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg","mpn":"AC-1",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock",
		 "priceValidUntil":"2026-12-31","hasMerchantReturnPolicy":{"@type":"MerchantReturnPolicy","returnPolicyCategory":"https://schema.org/MerchantReturnFiniteReturnWindow"},
		 "shippingDetails":{"@type":"OfferShippingDetails"},"itemCondition":"https://schema.org/NewCondition"}}
	</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-missing-merchant")
	if len(got) != 0 {
		missing, _ := got[0].Data["missing"].(map[string]int)
		t.Fatalf("expected no merchant gap for offers-nested priceValidUntil/hasMerchantReturnPolicy, got %v", missing)
	}
}

// allbirdsProductGroup mirrors a real Shopify storefront (allbirds.com): a ProductGroup
// carrying name/image/brand/description/productGroupID/variesBy and an offers block with
// price/priceCurrency/availability, whose only variant is a Product that carries nothing but
// a url — exactly the shape gocrawl's own shopify-flat-variant-product check tells store
// owners to adopt.
const allbirdsProductGroup = `{"@type":"ProductGroup","name":"Wool Runners",
	"image":"https://shop.test/wool-runners.jpg",
	"brand":{"@type":"Brand","name":"Allbirds"},
	"description":"A comfortable, sustainable sneaker.",
	"productGroupID":"WR-001",
	"variesBy":["size","color"],
	"offers":{"@type":"Offer","price":"98.00","priceCurrency":"USD","availability":"https://schema.org/InStock"},
	"hasVariant":[{"@type":"Product","url":"https://shop.test/products/wool-runners?variant=1"}]}`

func TestProductGroupWithHasVariantStillRaisesMerchantGap(t *testing.T) {
	// This is the defect from the real-world smoke test: a ProductGroup modeled with
	// hasVariant (the shape this project's own shopify-flat-variant-product check
	// recommends) must still surface the merchant-field gap. Before the fix, ProductGroup
	// had no merchant tier at all, and the sole Product node was exempt as a thin
	// hasVariant copy, so structured-missing-merchant never fired for this shape.
	html := `<html><head><script type="application/ld+json">` + allbirdsProductGroup + `</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/wool-runners": html})
	issues := structured.New().Analyze(context.Background(), res)

	merchant := findAll(issues, "structured-missing-merchant")
	if len(merchant) != 1 {
		t.Fatalf("expected 1 merchant-gap issue, got %d: %+v", len(merchant), merchant)
	}
	if merchant[0].Data["type"] != "ProductGroup" {
		t.Errorf("expected type ProductGroup, got %v", merchant[0].Data["type"])
	}
	missing, _ := merchant[0].Data["missing"].(map[string]int)
	for _, f := range []string{"gtin|gtin8|gtin12|gtin13|gtin14|mpn", "priceValidUntil|offers.priceValidUntil", "offers.shippingDetails", "hasMerchantReturnPolicy|offers.hasMerchantReturnPolicy", "offers.itemCondition"} {
		if missing[f] != 1 {
			t.Errorf("expected %q reported missing once, got %d (missing=%v)", f, missing[f], missing)
		}
	}

	// The thin hasVariant Product must remain exempt: it carries only a url, and
	// required-field-checking it would put a warning on every variant of every product.
	if is, ok := find(issues, "structured-missing-required"); ok {
		t.Errorf("expected no structured-missing-required issue (variant Product must stay exempt), got %+v", is)
	}
}

// wellStructuredProductGroup is the shape F1b targets: every hasVariant child carries the full
// merchant field set on its own Offer, which is exactly what Google's "fields on the variant"
// allowance describes and exactly what shopify-flat-variant-product tells owners to adopt.
const wellStructuredProductGroup = `{"@type":"ProductGroup","name":"Wool Runners",
	"image":"https://shop.test/wool-runners.jpg",
	"brand":{"@type":"Brand","name":"Allbirds"},
	"productGroupID":"WR-001",
	"variesBy":["size","color"],
	"hasVariant":[
		{"@type":"Product","url":"https://shop.test/products/wool-runners?variant=1","gtin13":"0012345678905",
		 "offers":{"@type":"Offer","price":"98.00","priceCurrency":"USD","availability":"https://schema.org/InStock",
		 "priceValidUntil":"2026-12-31","hasMerchantReturnPolicy":{"@type":"MerchantReturnPolicy"},
		 "shippingDetails":{"@type":"OfferShippingDetails"},"itemCondition":"https://schema.org/NewCondition"}}
	]}`

func TestProductGroupSatisfiedByVariantOffersRaisesNoMerchantGap(t *testing.T) {
	// F1b regression: when every hasVariant child carries the merchant fields on its own
	// Offer, the ProductGroup-level check must fall through to the variants rather than
	// reporting the group itself as missing everything.
	html := `<html><head><script type="application/ld+json">` + wellStructuredProductGroup + `</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/wool-runners": html})
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-missing-merchant")
	if len(got) != 0 {
		missing, _ := got[0].Data["missing"].(map[string]int)
		t.Fatalf("expected no merchant gap when every variant carries the full field set, got %v", missing)
	}
}

func TestNestedListProductsAreExemptFromEligibility(t *testing.T) {
	// A Shopify collection page lists products with only a name and a URL. Required-field
	// checking those would put a warning on every tile of every collection page.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"ItemList","itemListElement":[
			{"@type":"Product","name":"A","url":"https://shop.test/products/a"},
			{"@type":"Product","name":"B","url":"https://shop.test/products/b"}]}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Error("products nested in an ItemList must be exempt from eligibility checks")
	}
}

// The tests below each pin one row of the domain-reviewed tier corrections: a field that moved
// from recommended into required must now raise structured-missing-required when absent, on a
// node that otherwise carries every other required field for its type (so the assertion isolates
// the moved field rather than riding on some other gap).

func TestEventMissingLocationIsRequired(t *testing.T) {
	// Google requires location for every Event, including online ones (a VirtualLocation with
	// a url satisfies it) — it was wrongly filed as recommended.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Event","name":"Launch Party","startDate":"2026-10-01T18:00:00-07:00"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required")
	if !ok {
		t.Fatal("expected structured-missing-required for an Event with no location")
	}
	missing, _ := is.Data["missing"].([]string)
	if len(missing) != 1 || missing[0] != "location" {
		t.Errorf("expected exactly [location] missing, got %v", missing)
	}
}

func TestVideoObjectMissingDescriptionOrUploadDateIsRequired(t *testing.T) {
	// Google's required set for the video rich result is name + description + thumbnailUrl +
	// uploadDate; the table previously required only two of the four.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"VideoObject","name":"How it's made","thumbnailUrl":"https://shop.test/thumb.jpg"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required")
	if !ok {
		t.Fatal("expected structured-missing-required for a VideoObject with no description or uploadDate")
	}
	missing, _ := is.Data["missing"].([]string)
	want := map[string]bool{"description": true, "uploadDate": true}
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing fields, got %v", missing)
	}
	for _, m := range missing {
		if !want[m] {
			t.Errorf("unexpected missing field %q", m)
		}
	}
}

func TestRecipeMissingImageIsRequired(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Recipe","name":"Soup"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required")
	if !ok {
		t.Fatal("expected structured-missing-required for a Recipe with no image")
	}
	missing, _ := is.Data["missing"].([]string)
	if len(missing) != 1 || missing[0] != "image" {
		t.Errorf("expected exactly [image] missing, got %v", missing)
	}
}

func TestRecipeIngredientsAndInstructionsStayRecommended(t *testing.T) {
	// Counterpart to TestRecipeMissingImageIsRequired: recipeIngredient/recipeInstructions were
	// deliberately NOT moved, despite feeling central to a recipe, per Google's documentation.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Recipe","name":"Soup","image":"https://shop.test/soup.jpg"}
	</script></head><body></body></html>`)
	if is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Errorf("a Recipe missing only recipeIngredient/recipeInstructions must not raise structured-missing-required, got %+v", is.Data)
	}
}

func TestLocalBusinessMissingAddressIsRequired(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"LocalBusiness","name":"Corner Shop"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required")
	if !ok {
		t.Fatal("expected structured-missing-required for a LocalBusiness with no address")
	}
	missing, _ := is.Data["missing"].([]string)
	if len(missing) != 1 || missing[0] != "address" {
		t.Errorf("expected exactly [address] missing, got %v", missing)
	}
}

func TestOrganizationMissingLogoOrURLIsRequired(t *testing.T) {
	// Both required by Google's logo guidance.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Organization","name":"Acme"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required")
	if !ok {
		t.Fatal("expected structured-missing-required for an Organization with no logo/url")
	}
	missing, _ := is.Data["missing"].([]string)
	want := map[string]bool{"logo": true, "url": true}
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing fields, got %v", missing)
	}
	for _, m := range missing {
		if !want[m] {
			t.Errorf("unexpected missing field %q", m)
		}
	}
}

func TestProductGroupNeedsOnlyNameButRecommendsGrouping(t *testing.T) {
	// Google's product-variants documentation requires only name on a ProductGroup;
	// hasVariant and productGroupID are recommended, because variants may instead point up
	// at the group with isVariantOf. A bare named group is eligible, just weakly modelled.
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/wool-runners": `<html><head><script type="application/ld+json">
		{"@type":"ProductGroup","name":"Wool Runners"}
	</script></head><body></body></html>`})
	issues := structured.New().Analyze(context.Background(), res)
	if is, ok := find(issues, "structured-missing-required"); ok {
		t.Errorf("a named ProductGroup has every required field, got %+v", is)
	}
	var missing map[string]int
	for _, is := range findAll(issues, "structured-missing-recommended") {
		if is.Data["type"] == "ProductGroup" {
			missing, _ = is.Data["missing"].(map[string]int)
		}
	}
	if missing["hasVariant"] != 1 || missing["productGroupID"] != 1 {
		t.Errorf("expected hasVariant and productGroupID in the recommended rollup, got %v", missing)
	}
}

// inlineVariantGroup declares its variants inline (Google's single-page shape) but leaves each
// one short: no image, no identifier, and no size despite variesBy naming it. Two variants
// share those gaps, so the page must still count once per field; the third is a url-only stub,
// which must add nothing.
const inlineVariantGroup = `{"@type":"ProductGroup","name":"Wool Runners","productGroupID":"WR-001",
	"variesBy":["https://schema.org/size","color","https://schema.org/flavour"],
	"hasVariant":[
		{"@type":"Product","name":"Wool Runners — Red 9","color":"Red",
		 "offers":{"@type":"Offer","price":"98.00","priceCurrency":"USD"}},
		{"@type":"Product","name":"Wool Runners — Blue 9","color":"Blue",
		 "offers":{"@type":"Offer","price":"98.00","priceCurrency":"USD"}},
		{"@type":"Product","url":"https://shop.test/products/wool-runners?variant=3"}
	]}`

func TestInlineVariantGapsRollUpPerGroupType(t *testing.T) {
	html := `<html><head><script type="application/ld+json">` + inlineVariantGroup + `</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{
		"https://shop.test/products/a": html,
		"https://shop.test/products/b": html,
	})
	issues := structured.New().Analyze(context.Background(), res)

	got := findAll(issues, "structured-variant-incomplete")
	if len(got) != 1 {
		t.Fatalf("expected 1 rolled-up variant issue for 2 pages, got %d: %+v", len(got), got)
	}
	is := got[0]
	if is.Severity != analyze.Warning {
		t.Errorf("expected warning, got %q", is.Severity)
	}
	if is.Data["pages"] != 2 || is.Data[analyze.InstanceKey] != "ProductGroup" {
		t.Errorf("expected pages=2 instance=ProductGroup, got %v / %v", is.Data["pages"], is.Data[analyze.InstanceKey])
	}
	fields, _ := is.Data["fields"].([]string)
	want := []string{"image", "size", "sku|gtin|gtin8|gtin12|gtin13|gtin14"}
	if strings.Join(fields, ",") != strings.Join(want, ",") {
		t.Errorf("fields = %v, want %v (color is present; flavour is not a supported dimension)", fields, want)
	}
	missing, _ := is.Data["missing"].(map[string]int)
	for _, f := range want {
		if missing[f] != 2 {
			t.Errorf("missing[%q] = %d, want 2 (one per page, not one per variant)", f, missing[f])
		}
	}
	// Variants stay out of the per-page required check: that is what the rollup replaces.
	if is, ok := find(issues, "structured-missing-required"); ok {
		t.Errorf("inline variants must not raise per-page required findings, got %+v", is)
	}
}

func TestURLOnlyVariantStubsAreNotChecked(t *testing.T) {
	// Allbirds' shape, and Google's own multi-page example: variants served on other pages
	// are listed by url, with their markup on those pages.
	html := `<html><head><script type="application/ld+json">` + allbirdsProductGroup + `</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/wool-runners": html})
	issues := structured.New().Analyze(context.Background(), res)
	if _, ok := find(issues, "structured-missing-merchant"); !ok {
		t.Fatal("expected the merchant gap, otherwise this test passes vacuously")
	}
	if is, ok := find(issues, "structured-variant-incomplete"); ok {
		t.Errorf("url-only variant references are not incomplete variants, got %+v", is)
	}
}

func TestCompleteInlineVariantsAreSilent(t *testing.T) {
	html := `<html><head><script type="application/ld+json">
		{"@type":"ProductGroup","name":"Coat","variesBy":["https://schema.org/size"],"hasVariant":[
			{"@type":"Product","name":"Small coat","image":"https://shop.test/s.jpg","size":"small",
			 "gtin14":"98766051104214","offers":{"@type":"Offer","price":"39.99","priceCurrency":"USD"}}]}
	</script></head><body></body></html>`
	res := pages(t, "https://shop.test", map[string]string{"https://shop.test/products/coat": html})
	issues := structured.New().Analyze(context.Background(), res)
	if _, ok := find(issues, "structured-missing-recommended"); !ok {
		t.Fatal("expected some ProductGroup rollup, otherwise this test passes vacuously")
	}
	if is, ok := find(issues, "structured-variant-incomplete"); ok {
		t.Errorf("a variant carrying every required field is complete, got %+v", is)
	}
}

// TestWebSitePotentialActionNeverAppears covers the removal, not a move: potentialAction
// (Sitelinks Search Box markup) was deleted from the table outright, since Google deprecated
// the feature in November 2023. A WebSite carrying every remaining WebSite field (name, url)
// and no potentialAction must raise no finding at all for the type, and in particular no
// finding may ever mention "potentialAction" in its data, from any code this analyzer emits.
func TestWebSitePotentialActionNeverAppears(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"WebSite","name":"Shop","url":"https://shop.test/"}
	</script></head><body></body></html>`)
	issues := structured.New().Analyze(context.Background(), res)
	for _, is := range issues {
		for _, v := range is.Data {
			if s, ok := v.(string); ok && strings.Contains(s, "potentialAction") {
				t.Errorf("potentialAction must not appear anywhere in issue data, found in %+v", is)
			}
			if fields, ok := v.([]string); ok {
				for _, f := range fields {
					if f == "potentialAction" {
						t.Errorf("potentialAction must not appear as a missing field, found in %+v", is)
					}
				}
			}
		}
	}
	if is, ok := find(issues, "structured-missing-required"); ok {
		t.Errorf("a WebSite with name and url should raise no required gap, got %+v", is.Data)
	}
}
