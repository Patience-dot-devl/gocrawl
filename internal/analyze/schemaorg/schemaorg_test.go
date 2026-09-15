package schemaorg_test

import (
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/PuerkitoBio/goquery"
)

func parse(t *testing.T, html string) (schemaorg.Graph, []schemaorg.ParseError) {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	return schemaorg.Parse(doc)
}

func TestParseFlattensNestedObjects(t *testing.T) {
	g, errs := parse(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Product","name":"Tee",
		 "brand":{"@type":"Brand","name":"Acme"},
		 "offers":{"@type":"Offer","price":"19.99"}}
	</script></head><body></body></html>`)
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 nodes (Product, Brand, Offer), got %d", len(g.Nodes))
	}
	offers := g.OfType("Offer")
	if len(offers) != 1 {
		t.Fatalf("expected 1 Offer node, got %d", len(offers))
	}
	if offers[0].Path != "Product.offers" {
		t.Errorf("expected Offer path Product.offers, got %q", offers[0].Path)
	}
	if offers[0].Block != 0 {
		t.Errorf("expected Offer in block 0, got %d", offers[0].Block)
	}
}

func TestParseDescendsGraphAndArrays(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@graph":[{"@type":"WebSite"},{"@type":["BreadcrumbList","ItemList"]}]}
	</script></head><body></body></html>`)
	if !g.HasType("WebSite") || !g.HasType("BreadcrumbList") || !g.HasType("ItemList") {
		t.Errorf("expected WebSite, BreadcrumbList and ItemList, got %v", g.Types())
	}
	if len(g.Nodes) != 2 {
		t.Errorf("expected 2 nodes from @graph, got %d", len(g.Nodes))
	}
}

func TestParseRecordsBlockIndex(t *testing.T) {
	g, _ := parse(t, `<html><head>
		<script type="application/ld+json">{"@type":"Product","name":"A"}</script>
		<script type="application/ld+json">{"@type":"Product","name":"B"}</script>
	</head><body></body></html>`)
	products := g.OfType("Product")
	if len(products) != 2 {
		t.Fatalf("expected 2 Product nodes, got %d", len(products))
	}
	if products[0].Block != 0 || products[1].Block != 1 {
		t.Errorf("expected blocks 0 and 1, got %d and %d", products[0].Block, products[1].Block)
	}
}

func TestParseReportsInvalidBlock(t *testing.T) {
	g, errs := parse(t, `<html><head>
		<script type="application/ld+json">{ not json }</script>
		<script type="application/ld+json">{"@type":"Organization","name":"Acme"}</script>
	</head><body></body></html>`)
	if len(errs) != 1 {
		t.Fatalf("expected 1 parse error, got %d", len(errs))
	}
	if errs[0].Block != 0 {
		t.Errorf("expected the error on block 0, got %d", errs[0].Block)
	}
	if !g.HasType("Organization") {
		t.Error("expected the valid block to still be parsed")
	}
}

func TestParseSkipsEmptyBlocks(t *testing.T) {
	_, errs := parse(t, `<html><head><script type="application/ld+json">   </script></head><body></body></html>`)
	if len(errs) != 0 {
		t.Errorf("an empty block is not an error, got %v", errs)
	}
}

func TestResolveByID(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@graph":[{"@type":"Organization","@id":"https://x.test/#org","name":"Acme"},
		           {"@type":"WebSite","publisher":{"@id":"https://x.test/#org"}}]}
	</script></head><body></body></html>`)
	n, ok := g.Resolve("https://x.test/#org")
	if !ok {
		t.Fatal("expected to resolve the Organization by @id")
	}
	if !n.Is("Organization") {
		t.Errorf("expected an Organization node, got %v", n.Types)
	}
}

// TestTypesAreDeduplicatedAndStableAcrossParses guards against the map-iteration
// determinism bug where Graph.walk ranged directly over a map[string]any: sibling
// typed children of a JSON *object* (not array) would land in Graph.Nodes, and
// therefore in Types(), in a different order on every run. A top-level JSON array
// fixture doesn't exercise this — an array already has a real order — so this test
// nests the typed siblings inside an object's properties, and parses it many times
// to catch order flakiness that a single parse could pass by luck.
func TestTypesAreDeduplicatedAndStableAcrossParses(t *testing.T) {
	const html = `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee",
		 "brand":{"@type":"Brand","name":"Acme"},
		 "offers":{"@type":"Offer","price":"19.99"},
		 "manufacturer":{"@type":"Organization","name":"Acme Mfg"},
		 "additionalProperty":{"@type":"Offer","price":"9.99"}}
	</script></head><body></body></html>`

	first, _ := parse(t, html)
	want := first.Types()

	if len(want) != 4 {
		t.Fatalf("expected 4 de-duplicated types (Product, Brand, Offer, Organization), got %v", want)
	}
	seen := make(map[string]bool, len(want))
	for _, ty := range want {
		if seen[ty] {
			t.Fatalf("expected de-duplicated types, got a repeat of %q in %v", ty, want)
		}
		seen[ty] = true
	}

	for i := 0; i < 50; i++ {
		g, _ := parse(t, html)
		got := g.Types()
		if !equalStrings(got, want) {
			t.Fatalf("run %d: Types() order was not stable: got %v, want %v", i, got, want)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStrResolvesDottedPath(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD"}}
	</script></head><body></body></html>`)
	p := g.OfType("Product")[0]
	if got := g.Str(p, "offers.price"); got != "19.99" {
		t.Errorf("expected 19.99, got %q", got)
	}
	if got := g.Str(p, "offers.priceCurrency"); got != "USD" {
		t.Errorf("expected USD, got %q", got)
	}
	if got := g.Str(p, "offers.availability"); got != "" {
		t.Errorf("expected empty for an absent path, got %q", got)
	}
}

func TestStrCoercesNumbers(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":19.99}}
	</script></head><body></body></html>`)
	p := g.OfType("Product")[0]
	if got := g.Str(p, "offers.price"); got != "19.99" {
		t.Errorf("expected a numeric price rendered as 19.99, got %q", got)
	}
}

func TestPathFollowsIDReference(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@graph":[{"@type":"Brand","@id":"https://x.test/#brand","name":"Acme"},
		           {"@type":"Product","name":"Tee","brand":{"@id":"https://x.test/#brand"}}]}
	</script></head><body></body></html>`)
	p := g.OfType("Product")[0]
	if got := g.Str(p, "brand.name"); got != "Acme" {
		t.Errorf("expected the @id reference to resolve to Acme, got %q", got)
	}
}

func TestPathTraversesArrays(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","offers":[{"@type":"Offer","price":"10.00"},{"@type":"Offer","price":"20.00"}]}
	</script></head><body></body></html>`)
	p := g.OfType("Product")[0]
	got := g.Strs(p, "offers.price")
	if len(got) != 2 || got[0] != "10.00" || got[1] != "20.00" {
		t.Errorf("expected both offer prices, got %v", got)
	}
}

func TestHasValueTreatsEmptyAsAbsent(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","sku":"","description":"  ","image":[],"brand":{"name":"Acme"}}
	</script></head><body></body></html>`)
	p := g.OfType("Product")[0]
	for _, path := range []string{"sku", "description", "image", "gtin"} {
		if g.HasValue(p, path) {
			t.Errorf("expected %q to count as absent", path)
		}
	}
	if !g.HasValue(p, "brand") {
		t.Error("expected a populated brand object to count as present")
	}
}

func TestNodesAtReturnsTypedChildren(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		{"@type":"ProductGroup","name":"Tee","hasVariant":[{"@type":"Product","name":"S"},{"@type":"Product","name":"M"}]}
	</script></head><body></body></html>`)
	pg := g.OfType("ProductGroup")[0]
	if got := g.NodesAt(pg, "hasVariant"); len(got) != 2 {
		t.Errorf("expected 2 variant nodes, got %d", len(got))
	}
}
