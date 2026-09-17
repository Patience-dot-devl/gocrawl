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
