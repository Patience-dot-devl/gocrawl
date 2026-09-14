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

func TestTypesAreDeduplicatedInDocumentOrder(t *testing.T) {
	g, _ := parse(t, `<html><head><script type="application/ld+json">
		[{"@type":"Product","name":"A"},{"@type":"Product","name":"B"},{"@type":"Organization","name":"C"}]
	</script></head><body></body></html>`)
	got := g.Types()
	if len(got) != 2 || got[0] != "Product" || got[1] != "Organization" {
		t.Errorf("expected [Product Organization], got %v", got)
	}
}
