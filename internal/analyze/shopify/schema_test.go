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
