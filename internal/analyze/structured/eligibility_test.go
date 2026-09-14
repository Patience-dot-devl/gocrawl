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
	missing, _ := is.Data["missing"].([]string)
	want := map[string]bool{"offers.price": true, "offers.priceCurrency": true, "offers.availability": true}
	if len(missing) != 3 {
		t.Fatalf("expected 3 missing offer fields, got %v", missing)
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
		t.Error("a Product with all five required fields should raise no required gap")
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
	for _, f := range []string{"gtin|gtin8|gtin12|gtin13|gtin14|mpn", "priceValidUntil", "offers.shippingDetails", "hasMerchantReturnPolicy"} {
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
	for _, f := range []string{"gtin|gtin8|gtin12|gtin13|gtin14|mpn", "priceValidUntil", "offers.shippingDetails", "hasMerchantReturnPolicy"} {
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
