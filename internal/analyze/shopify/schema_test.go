package shopify_test

import (
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
)

func TestSchemaAppConflict(t *testing.T) {
	// The theme emits its own Product, then an SEO app injects a second one.
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"19.99"}}</script>
		<script src="https://cdn.shopify.com/extensions/abc/json-ld-for-seo/assets/app.js"></script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"24.99"}}</script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	is, ok := find(run(t, res), "shopify-schema-app-conflict")
	if !ok {
		t.Fatal("expected shopify-schema-app-conflict")
	}
	if is.Severity != analyze.Error {
		t.Errorf("expected error severity, got %q", is.Severity)
	}
	sources, _ := is.Data["sources"].([]string)
	if len(sources) != 2 {
		t.Fatalf("expected two attributed sources, got %v", sources)
	}
	// Sorted, not document order: the theme block comes first on the page, and a report whose
	// Data follows page order would churn in gocrawl compare when an app moves its script tag.
	if sources[0] != "JSON-LD for SEO" || sources[1] != "theme" {
		t.Errorf("expected sources sorted [JSON-LD for SEO theme], got %v", sources)
	}
}

func TestTwoThemeBlocksAreNotAConflict(t *testing.T) {
	// A theme emitting Product and BreadcrumbList in two blocks is normal, and two Product
	// blocks from one source is a duplicate (the structured analyzer's finding), not an app
	// conflict.
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected — otherwise this test passes vacuously")
	}
	if _, ok := find(issues, "shopify-schema-app-conflict"); ok {
		t.Error("two blocks attributed to the same source are not an app conflict")
	}
}

func TestAppAttributedByBlockIDAttribute(t *testing.T) {
	// Several apps mark their own block rather than loading a recognizable script first.
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
		<script type="application/ld+json" id="schemaplus-product">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	is, ok := find(run(t, res), "shopify-schema-app-conflict")
	if !ok {
		t.Fatal("expected a conflict attributed via the block's own id")
	}
	sources, _ := is.Data["sources"].([]string)
	var sawApp bool
	for _, s := range sources {
		if s == "Schema Plus" {
			sawApp = true
		}
	}
	if !sawApp {
		t.Errorf("expected Schema Plus among the sources, got %v", sources)
	}
}

func TestClientInjectedSchemaWarning(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script src="https://cdn.shopify.com/extensions/abc/searchpie/assets/app.js"></script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	is, ok := find(run(t, res), "shopify-schema-client-injected")
	if !ok {
		t.Fatal("expected shopify-schema-client-injected")
	}
	if is.Data["app"] != "SearchPie" {
		t.Errorf("expected the app named in data, got %v", is.Data["app"])
	}
}

func TestNoClientInjectionWarningWhenSchemaIsPresent(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script src="https://cdn.shopify.com/extensions/abc/searchpie/assets/app.js"></script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected — otherwise this test passes vacuously")
	}
	if _, ok := find(issues, "shopify-schema-client-injected"); ok {
		t.Error("no warning is needed when the raw HTML already carries JSON-LD")
	}
}

const variantSelector = `<variant-radios><select name="id">
	<option value="1">Small</option><option value="2">Medium</option></select></variant-radios>`

func TestFlatVariantProduct(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}</script>
	</head><body><h1>Tee</h1>` + variantSelector + `</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	is, ok := find(run(t, res), "shopify-flat-variant-product")
	if !ok {
		t.Fatal("expected shopify-flat-variant-product")
	}
	if is.Data["variants"] != 2 {
		t.Errorf("expected variants=2, got %v", is.Data["variants"])
	}
}

func TestProductGroupSuppressesFlatVariantWarning(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"ProductGroup","name":"Tee","productGroupID":"T",
		 "hasVariant":[{"@type":"Product","name":"Tee S"},{"@type":"Product","name":"Tee M"}]}</script>
	</head><body><h1>Tee</h1>` + variantSelector + `</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected — otherwise this test passes vacuously")
	}
	if _, ok := find(issues, "shopify-flat-variant-product"); ok {
		t.Error("a ProductGroup already models the variants")
	}
}

func TestSingleVariantProductIsNotFlagged(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1><select name="id"><option value="1">Default</option></select></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected — otherwise this test passes vacuously")
	}
	if _, ok := find(issues, "shopify-flat-variant-product"); ok {
		t.Error("a product with one variant has nothing to model")
	}
}

func TestSingleOfferForMultiplePrices(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}</script>
	</head><body><h1>Tee</h1>` + variantSelector + `
		<script type="application/json" id="ProductJson-product-template">
			{"variants":[{"id":1,"price":1999},{"id":2,"price":2499}]}
		</script></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	is, ok := find(run(t, res), "shopify-single-offer-range")
	if !ok {
		t.Fatal("expected shopify-single-offer-range")
	}
	if is.Data["prices"] != 2 {
		t.Errorf("expected prices=2, got %v", is.Data["prices"])
	}
}

func TestUniformVariantPricesNeedNoRange(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}</script>
	</head><body><h1>Tee</h1>` + variantSelector + `
		<script type="application/json" id="ProductJson-product-template">
			{"variants":[{"id":1,"price":1999},{"id":2,"price":1999}]}
		</script></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected — otherwise this test passes vacuously")
	}
	if _, ok := find(issues, "shopify-single-offer-range"); ok {
		t.Error("variants that all cost the same are correctly described by one Offer")
	}
}

func TestVariantCountTakesMaxAcrossSelectorsNotSum(t *testing.T) {
	// A single variant, rendered twice by two different selector shapes: a hidden input for
	// the buy form and a data-variant-id element for a swatch. max(1,1) = 1, below the >= 2
	// gate. A regression to summing across selectors would read 1+1 = 2 and trip it on a
	// product with nothing to model.
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1>
		<input type="hidden" name="id" value="1">
		<div data-variant-id="1">In stock</div>
	</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected — otherwise this test passes vacuously")
	}
	if _, ok := find(issues, "shopify-flat-variant-product"); ok {
		t.Error("one variant rendered by two selectors is still one variant, not two")
	}
}

func TestPlaceholderOptionIsNotCountedAsVariant(t *testing.T) {
	// A leading "Choose an option" placeholder with no value is not a variant. A single real
	// variant plus that placeholder must not trip the >= 2 gate.
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1>
		<select name="id"><option value="">Choose an option</option><option value="1">Default</option></select>
	</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": html})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected — otherwise this test passes vacuously")
	}
	if _, ok := find(issues, "shopify-flat-variant-product"); ok {
		t.Error("a placeholder option with no value is not a second variant")
	}
}

// stickyCartForms renders the buy form twice, as themes with a sticky add-to-cart bar do: two
// form[action="/cart/add"], each with a hidden input carrying the same selected variant id.
const stickyCartForms = `
	<form action="/cart/add" method="post"><input type="hidden" name="id" value="40001">
		<button type="submit">Add to cart</button></form>
	<div data-variant-id="40001" class="price">EUR 49,00</div>
	<form action="/cart/add" method="post" class="sticky-atc"><input type="hidden" name="id" value="40001">
		<button type="submit">Add to cart</button></form>`

func TestRepeatedCartFormIsOneVariant(t *testing.T) {
	// Main form plus sticky form repeat one variant id. Counting elements would read 2 and
	// flag a single-variant product; counting distinct values reads 1.
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Cleanser","sku":"C-1",
		 "offers":{"@type":"Offer","price":"49.00","priceCurrency":"EUR","url":"https://shop.test/products/cleanser"}}</script>
	</head><body><h1>Cleanser</h1>` + stickyCartForms + `</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/cleanser": html})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected — otherwise this test passes vacuously")
	}
	if is, ok := find(issues, "shopify-flat-variant-product"); ok {
		t.Errorf("one variant id repeated across two cart forms is one variant, got %+v", is)
	}
}

// sizeOffers is a flat Product with one Offer per size, each linking to its ?variant= URL, as
// Shopify themes emit it. The DOM size picker is markup no variantSelectors entry matches.
const sizeOffers = `<script type="application/ld+json">{"@context":"https://schema.org","@type":"Product",
	"name":"Cleanser","url":"https://shop.test/products/cleanser","sku":"C-1","brand":{"@type":"Brand","name":"Shop"},
	"offers":[
		{"@type":"Offer","sku":"C-50","gtin12":123456789012,"price":"19.00","priceCurrency":"EUR","availability":"https://schema.org/InStock","url":"https://shop.test/products/cleanser?variant=40001"},
		{"@type":"Offer","sku":"C-150","gtin12":123456789013,"price":"49.00","priceCurrency":"EUR","availability":"https://schema.org/InStock","url":"https://shop.test/products/cleanser?variant=40002"},
		{"@type":"Offer","sku":"C-250","gtin12":123456789014,"price":"69.00","priceCurrency":"EUR","availability":"https://schema.org/InStock","url":"/products/cleanser?variant=40003"}
	]}</script>`

func TestFlatVariantCountsOfferVariantURLs(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		` + sizeOffers + `
	</head><body><h1>Cleanser</h1>
		<div class="size-picker"><button class="size-chip">50 ml</button><button class="size-chip">150 ml</button><button class="size-chip">250 ml</button></div>
		` + stickyCartForms + `</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/cleanser": html})
	is, ok := find(run(t, res), "shopify-flat-variant-product")
	if !ok {
		t.Fatal("expected shopify-flat-variant-product: the offers name three variants")
	}
	if is.Data["variants"] != 3 {
		t.Errorf("expected variants=3, got %v", is.Data["variants"])
	}
	if is.Data["source"] != "offers" {
		t.Errorf("expected source=offers, got %v", is.Data["source"])
	}
}

func TestFlatVariantCountsDistinctSelectOptions(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","url":"https://shop.test/products/tee"}}</script>
	</head><body><h1>Tee</h1>
		<select name="id"><option value="1">S</option><option value="2">M</option><option value="3">L</option></select>
	</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/tee": html})
	is, ok := find(run(t, res), "shopify-flat-variant-product")
	if !ok {
		t.Fatal("expected shopify-flat-variant-product")
	}
	if is.Data["variants"] != 3 {
		t.Errorf("expected variants=3, got %v", is.Data["variants"])
	}
	if is.Data["source"] != "dom" {
		t.Errorf("expected source=dom, got %v", is.Data["source"])
	}
}

func TestFlatVariantSourceWhenDOMAndOffersAgree(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		` + sizeOffers + `
	</head><body><h1>Cleanser</h1>
		<select name="id"><option value="40001">50 ml</option><option value="40002">150 ml</option><option value="40003">250 ml</option></select>
	</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/cleanser": html})
	is, ok := find(run(t, res), "shopify-flat-variant-product")
	if !ok {
		t.Fatal("expected shopify-flat-variant-product")
	}
	if is.Data["variants"] != 3 || is.Data["source"] != "dom+offers" {
		t.Errorf("expected variants=3 source=dom+offers, got %v %v", is.Data["variants"], is.Data["source"])
	}
}

func TestSwatchGridCountsDistinctDataVariantIDs(t *testing.T) {
	// Two variants, each id attached to a swatch and a thumbnail. The id lives in the
	// data-variant-id attribute itself; the count is distinct ids (2), not elements (4).
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
	</head><body><h1>Tee</h1>
		<ul class="swatches"><li data-variant-id="1">Red</li><li data-variant-id="2">Blue</li></ul>
		<ul class="thumbs"><li data-variant-id="1"><img src="r.jpg" alt=""></li><li data-variant-id="2"><img src="b.jpg" alt=""></li></ul>
	</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/tee": html})
	is, ok := find(run(t, res), "shopify-flat-variant-product")
	if !ok {
		t.Fatal("expected shopify-flat-variant-product")
	}
	if is.Data["variants"] != 2 || is.Data["source"] != "dom" {
		t.Errorf("expected variants=2 source=dom, got %v %v", is.Data["variants"], is.Data["source"])
	}
}

func TestListedProductOffersAreNotThisPagesVariants(t *testing.T) {
	// A "you may also like" rail nests other products whose offers carry their own ?variant=
	// URLs. Those are not variants of the product this page sells.
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Cleanser",
		 "offers":{"@type":"Offer","price":"49.00","priceCurrency":"EUR","url":"https://shop.test/products/cleanser?variant=40001"},
		 "isRelatedTo":[
		  {"@type":"Product","name":"Toner","offers":{"@type":"Offer","url":"https://shop.test/products/toner?variant=50001"}},
		  {"@type":"Product","name":"Serum","offers":{"@type":"Offer","url":"https://shop.test/products/serum?variant=60001"}}
		 ]}</script>
	</head><body><h1>Cleanser</h1>` + stickyCartForms + `</body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/cleanser": html})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected — otherwise this test passes vacuously")
	}
	if is, ok := find(issues, "shopify-flat-variant-product"); ok {
		t.Errorf("related products' offers are not this page's variants, got %+v", is)
	}
}

// TestNoClientInjectionWarningOnUtilityPages pins the template gate. Apps load sitewide, so the
// cart and search pages carry the same app script as a product page; no store is expected to
// mark those up, and a warning there would be noise on every crawl.
func TestNoClientInjectionWarningOnUtilityPages(t *testing.T) {
	html := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script src="https://cdn.shopify.com/extensions/abc/searchpie/assets/app.js"></script>
	</head><body><h1>Cart</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/cart": html})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected — otherwise this test passes vacuously")
	}
	if is, ok := find(issues, "shopify-schema-client-injected"); ok {
		t.Errorf("a utility page is not expected to carry structured data, got %+v", is)
	}
}
