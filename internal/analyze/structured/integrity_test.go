package structured_test

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/structured"
)

func TestDuplicateProductAcrossBlocks(t *testing.T) {
	res := page(t, `<html><head>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","sku":"T-1"}</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","sku":"T-1"}</script>
	</head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-duplicate-type")
	if !ok {
		t.Fatal("expected structured-duplicate-type for two Product blocks")
	}
	if is.Data["type"] != "Product" {
		t.Errorf("expected type Product, got %v", is.Data["type"])
	}
	if is.Data["blocks"] != 2 {
		t.Errorf("expected blocks=2, got %v", is.Data["blocks"])
	}
}

func TestDuplicateWithinOneBlockIsNotFlagged(t *testing.T) {
	// Two Products in one block is a deliberate modelling choice (a comparison page, a
	// bundle), not two sources fighting over the page.
	res := page(t, `<html><head><script type="application/ld+json">
		[{"@type":"Product","name":"A"},{"@type":"Product","name":"B"}]
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-duplicate-type"); ok {
		t.Error("two Products in one block must not be flagged as duplicates")
	}
}

func TestConflictingValueBetweenBlocks(t *testing.T) {
	res := page(t, `<html><head>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"19.99"}}</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"24.99"}}</script>
	</head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-conflicting-value")
	if !ok {
		t.Fatal("expected structured-conflicting-value for disagreeing prices")
	}
	if is.Severity != analyze.Error {
		t.Errorf("expected error severity, got %q", is.Severity)
	}
	if is.Data["field"] != "offers.price" {
		t.Errorf("expected field offers.price, got %v", is.Data["field"])
	}
	values, _ := is.Data["values"].([]string)
	if len(values) != 2 {
		t.Errorf("expected both conflicting values, got %v", values)
	}
}

func TestAgreeingDuplicateHasNoConflict(t *testing.T) {
	res := page(t, `<html><head>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","sku":"T-1"}</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","sku":"T-1"}</script>
	</head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-conflicting-value"); ok {
		t.Error("identical duplicates disagree about nothing")
	}
}

func TestUnresolvedIDReference(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"WebSite","name":"Shop","publisher":{"@id":"https://shop.test/#missing"}}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-unresolved-id")
	if !ok {
		t.Fatal("expected structured-unresolved-id for a dangling reference")
	}
	if is.Data["id"] != "https://shop.test/#missing" {
		t.Errorf("expected the dangling id in data, got %v", is.Data["id"])
	}
}

func TestResolvedIDReferenceIsSilent(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@graph":[{"@type":"Organization","@id":"https://shop.test/#org","name":"Shop"},
		           {"@type":"WebSite","name":"Shop","publisher":{"@id":"https://shop.test/#org"}}]}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-unresolved-id"); ok {
		t.Error("a reference whose target is on the page must not be flagged")
	}
}
