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

func TestMultipleUnresolvedIDsOnOneNodeAreDeterministic(t *testing.T) {
	// One node carries two different dangling references, on two different properties.
	// n.Props is a map, and Go randomizes map iteration order per run, so a naive range
	// over it would make both the order of the emitted issues and, via the seen dedup,
	// which property gets reported a coin flip. Assert both fire and pin the order
	// explicitly (sorted by property name: "brand" before "publisher") so a regression to
	// map-range order breaks this test rather than passing by luck.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee",
		 "brand":{"@id":"https://shop.test/#brand-missing"},
		 "publisher":{"@id":"https://shop.test/#publisher-missing"}}
	</script></head><body></body></html>`)
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-unresolved-id")
	if len(got) != 2 {
		t.Fatalf("expected 2 unresolved-id issues, got %d: %+v", len(got), got)
	}
	if got[0].Data["property"] != "brand" || got[0].Data["id"] != "https://shop.test/#brand-missing" {
		t.Errorf("expected the brand reference first, got %+v", got[0].Data)
	}
	if got[1].Data["property"] != "publisher" || got[1].Data["id"] != "https://shop.test/#publisher-missing" {
		t.Errorf("expected the publisher reference second, got %+v", got[1].Data)
	}
}

func TestAbsentValueOnOneBlockIsNotADisagreement(t *testing.T) {
	// The first block declares an offers.price; the second says nothing about price at
	// all. Silence is not disagreement — disagreement requires two distinct non-empty
	// values, and a missing field on one side is neither.
	res := page(t, `<html><head>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"19.99"}}</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee"}</script>
	</head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-conflicting-value"); ok {
		t.Error("an absent value on one block must not be reported as a conflict")
	}
}

func TestRelativeURLInMarkup(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"/cdn/shop/t.jpg","url":"/products/tee"}
	</script></head><body></body></html>`)
	issues := structured.New().Analyze(context.Background(), res)
	got := findAll(issues, "structured-relative-url")
	if len(got) != 2 {
		t.Fatalf("expected a finding for each of image and url, got %d", len(got))
	}
}

func TestAbsoluteAndProtocolRelativeURLsAreFine(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"//cdn.shop.test/t.jpg","url":"https://shop.test/products/tee"}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-relative-url"); ok {
		t.Error("absolute and protocol-relative URLs are both resolvable")
	}
}

func TestInvalidDate(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"BlogPosting","headline":"Hi","datePublished":"14/09/2026"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-invalid-date")
	if !ok {
		t.Fatal("expected structured-invalid-date for a non-ISO date")
	}
	if is.Data["property"] != "datePublished" {
		t.Errorf("expected property datePublished, got %v", is.Data["property"])
	}
}

func TestISODatesAccepted(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"BlogPosting","headline":"Hi","datePublished":"2026-09-14",
		 "dateModified":"2026-09-14T08:30:00+02:00"}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-invalid-date"); ok {
		t.Error("a date-only and an RFC3339 timestamp are both valid ISO 8601")
	}
}

func TestMalformedPrice(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"$1,299.00","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-malformed-price")
	if !ok {
		t.Fatal("expected structured-malformed-price for a formatted price string")
	}
	if is.Data["value"] != "$1,299.00" {
		t.Errorf("expected the offending value in data, got %v", is.Data["value"])
	}
}

func TestNumericAndPlainStringPricesAccepted(t *testing.T) {
	for _, price := range []string{`19.99`, `"19.99"`, `"1299"`} {
		res := page(t, `<html><head><script type="application/ld+json">
			{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
			 "offers":{"@type":"Offer","price":`+price+`,"priceCurrency":"USD","availability":"https://schema.org/InStock"}}
		</script></head><body></body></html>`)
		if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-malformed-price"); ok {
			t.Errorf("price %s is well formed", price)
		}
	}
}

func TestPriceMismatchWithPage(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body><p class="price">$24.99</p><button>Add to cart</button></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch")
	if !ok {
		t.Fatal("expected structured-price-mismatch")
	}
	if is.Data["markup"] != "19.99" || is.Data["page"] != "24.99" {
		t.Errorf("expected markup 19.99 vs page 24.99, got %v / %v", is.Data["markup"], is.Data["page"])
	}
}

func TestPriceMatchIsSilent(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"24.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body><p class="price">$24.99</p><button>Add to cart</button></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch"); ok {
		t.Error("a matching price must not be flagged")
	}
}

func TestAmbiguousPageStaysSilent(t *testing.T) {
	// A sale price next to a struck-through original, or a variant selector, puts more than
	// one price on the page. There is no single on-page price to disagree with.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body><s>$29.99</s><p class="price">$24.99</p><button>Add to cart</button></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch"); ok {
		t.Error("a page showing two prices is ambiguous, not wrong")
	}
}

func TestRelativeURLDuplicateValueAcrossPropertiesCollapses(t *testing.T) {
	// Two different properties (image, url) holding the exact same bad value must collapse
	// into one finding: the add closure de-duplicates on code + the offending value, not on
	// which property carried it.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"/same-path","url":"/same-path"}
	</script></head><body></body></html>`)
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-relative-url")
	if len(got) != 1 {
		t.Fatalf("expected the same bad value on two properties to collapse into one finding, got %d: %+v", len(got), got)
	}
}

func TestNormalizePriceFormats(t *testing.T) {
	// Exercises normalizePrice's separator disambiguation through the public analyzer
	// behaviour: for each case, markup and on-page price are the same underlying amount
	// written in a different format, so the check must stay silent. The two ambiguous forms
	// carry a markup price that genuinely differs, to prove the guard suppresses the check
	// entirely rather than coincidentally agreeing.
	cases := []struct {
		name        string
		markupPrice string // offers.price in JSON-LD, already a bare decimal
		pagePrice   string // as it appears in the page body
	}{
		{"us decimal point", "19.99", "$19.99"},
		{"eu decimal comma", "19.99", "€19,99"},
		{"us thousands dot then decimal", "1299.00", "$1,299.00"},
		{"eu thousands dot then decimal comma", "1299.00", "€1.299,00"},
		{"dot thousands grouping only", "1234567", "1.234.567 EUR"},
		{"comma thousands grouping only", "1234567", "1,234,567 USD"},
		{"ambiguous single comma stays silent", "999.00", "$1,299"},
		{"ambiguous single dot stays silent", "999.00", "€1.299"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := page(t, `<html><head><script type="application/ld+json">
				{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
				 "offers":{"@type":"Offer","price":"`+tc.markupPrice+`","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
			</script></head><body><p class="price">`+tc.pagePrice+`</p><button>Add to cart</button></body></html>`)
			if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch"); ok {
				t.Errorf("markup=%s page=%s: expected no mismatch", tc.markupPrice, tc.pagePrice)
			}
		})
	}
}

func TestNormalizePriceStillCatchesRealMismatches(t *testing.T) {
	// Guards against a fix that makes the check trivially silent for every format: a genuine
	// disagreement expressed in EU notation must still fire.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body><p class="price">€24,99</p><button>Add to cart</button></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch")
	if !ok {
		t.Fatal("expected structured-price-mismatch for a genuine EU-notation disagreement")
	}
	if is.Data["markup"] != "19.99" || is.Data["page"] != "24.99" {
		t.Errorf("expected markup 19.99 vs page 24.99, got %v / %v", is.Data["markup"], is.Data["page"])
	}
}

func TestEUPriceMatchIsSilent(t *testing.T) {
	// End-to-end: an EU-formatted page whose single on-page price genuinely matches the
	// markup must raise no structured-price-mismatch.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"1299.00","priceCurrency":"EUR","availability":"https://schema.org/InStock"}}
	</script></head><body><p class="price">€1.299,00</p><button>Add to cart</button></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch"); ok {
		t.Error("a matching EU-formatted price must not be flagged")
	}
}

func TestAmbiguousPagePriceSuppressesCheckEvenAmongMultiplePrices(t *testing.T) {
	// An ambiguous price must not be silently dropped from the distinct-price count: if it
	// were, a page that actually shows two prices (one ambiguous, one not) could look like
	// it shows exactly one, and the mismatch check would fire on a comparison it has no
	// business making. distinctPagePrices must give up on the whole page instead.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body><p class="price">$19.99</p><s>$1,299</s><button>Add to cart</button></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch"); ok {
		t.Error("an ambiguous price anywhere on the page must suppress the mismatch check")
	}
}
