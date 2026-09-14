package shopify_test

import "testing"

const shopifyShell = `<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>`

func TestIndexableUtilityPage(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":             shopifyHome,
		"https://shop.test/search?q=tee": `<html><head>` + shopifyShell + `</head><body>Results</body></html>`,
	})
	is, ok := find(run(t, res), "shopify-indexable-utility")
	if !ok {
		t.Fatal("expected shopify-indexable-utility for a crawlable /search page")
	}
	if is.URL != "https://shop.test/search?q=tee" {
		t.Errorf("expected the finding on the utility URL, got %q", is.URL)
	}
}

func TestNoindexedUtilityPageIsFine(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/": shopifyHome,
		"https://shop.test/search?q=tee": `<html><head>` + shopifyShell +
			`<meta name="robots" content="noindex,follow"></head><body>Results</body></html>`,
	})
	issues := run(t, res)
	// Guard against a vacuous pass: if detection failed, Analyze returns nil and the absence
	// of shopify-indexable-utility below would prove nothing.
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected, otherwise this test passes vacuously")
	}
	if _, ok := find(issues, "shopify-indexable-utility"); ok {
		t.Error("a noindexed utility page is already handled")
	}
}

func TestUtilityNoindexViaHeader(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":     shopifyHome,
		"https://shop.test/cart": `<html><head>` + shopifyShell + `</head><body>Cart</body></html>`,
	})
	for _, p := range res.Pages {
		if p.FinalURL == "https://shop.test/cart" {
			p.Header = map[string][]string{"X-Robots-Tag": {"noindex"}}
		}
	}
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected, otherwise this test passes vacuously")
	}
	if _, ok := find(issues, "shopify-indexable-utility"); ok {
		t.Error("X-Robots-Tag: noindex must count the same as a meta robots tag")
	}
}

func TestIndexableFacetedCollection(t *testing.T) {
	faceted := `<html><head>` + shopifyShell +
		`<link rel="canonical" href="https://shop.test/collections/all?sort_by=price-asc"></head><body>Grid</body></html>`
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/": shopifyHome,
		"https://shop.test/collections/all?sort_by=price-asc": faceted,
	})
	is, ok := find(run(t, res), "shopify-indexable-facet")
	if !ok {
		t.Fatal("expected shopify-indexable-facet for a self-canonical sorted collection")
	}
	if is.Data["parameter"] != "sort_by" {
		t.Errorf("expected parameter sort_by, got %v", is.Data["parameter"])
	}
}

func TestFacetCanonicalisedToUnfilteredCollectionIsFine(t *testing.T) {
	faceted := `<html><head>` + shopifyShell +
		`<link rel="canonical" href="https://shop.test/collections/all"></head><body>Grid</body></html>`
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/": shopifyHome,
		"https://shop.test/collections/all?sort_by=price-asc": faceted,
	})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected, otherwise this test passes vacuously")
	}
	if _, ok := find(issues, "shopify-indexable-facet"); ok {
		t.Error("a facet canonicalised to the unfiltered collection is correctly handled")
	}
}

// TestPolicyPageNeverFlaggedAsUtility pins the reason TemplatePolicy exists as a template
// separate from TemplateUtility: refund/privacy/terms pages are meant to be indexed, so
// seoIssues must never fire shopify-indexable-utility on them. This is the guardrail for a
// classification decision made in Task 9 — a future change that folded TemplatePolicy back
// into TemplateUtility would false-positive on every store's policy pages, and this test
// would catch it.
func TestPolicyPageNeverFlaggedAsUtility(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":                       shopifyHome,
		"https://shop.test/policies/refund-policy": `<html><head>` + shopifyShell + `</head><body>Refunds</body></html>`,
	})
	issues := run(t, res)
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected, otherwise this test passes vacuously")
	}
	if is, ok := find(issues, "shopify-indexable-utility"); ok {
		t.Errorf("a policy page must never be flagged as an indexable utility page, got %+v", is)
	}
}

// TestFacetParamIsDeterministicWithMultipleParams pins facetParam's fix for the nondeterminism
// in ranging over a URL's query map: a collection URL that carries two faceting parameters at
// once must always report the same one, in facetParams' declared order (sort_by before
// filter.), regardless of Go's randomized map iteration order. Reports are diffed between
// crawls, so a nondeterministic Data value here would show up as a spurious diff on an
// unchanged page.
func TestFacetParamIsDeterministicWithMultipleParams(t *testing.T) {
	url := "https://shop.test/collections/all?sort_by=price-asc&filter.v.price.gte=10"
	faceted := `<html><head>` + shopifyShell + `</head><body>Grid</body></html>`
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/": shopifyHome,
		url:                  faceted,
	})
	for i := 0; i < 20; i++ {
		is, ok := find(run(t, res), "shopify-indexable-facet")
		if !ok {
			t.Fatal("expected shopify-indexable-facet")
		}
		if is.Data["parameter"] != "sort_by" {
			t.Fatalf("expected the deterministic first match sort_by, got %v", is.Data["parameter"])
		}
	}
}
