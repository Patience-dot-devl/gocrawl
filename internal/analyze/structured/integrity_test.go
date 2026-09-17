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

// TestListTilesAcrossBlocksAreNotDuplicateProducts pins topLevelOfType's exemption. A theme and
// an app each emitting an ItemList of product tiles is two lists, and the tiles inside them are
// thin copies, not two sources fighting over one page-level Product.
func TestListTilesAcrossBlocksAreNotDuplicateProducts(t *testing.T) {
	list := `{"@type":"ItemList","itemListElement":[{"@type":"ListItem","position":1,
		"item":{"@type":"Product","name":"Tee","url":"https://shop.test/products/tee"}}]}`
	res := page(t, `<html><head>
		<script type="application/ld+json">`+list+`</script>
		<script type="application/ld+json">`+list+`</script>
	</head><body></body></html>`)
	issues := structured.New().Analyze(context.Background(), res)
	for _, is := range issues {
		if is.Code == "structured-duplicate-type" && is.Data["type"] == "Product" {
			t.Errorf("product tiles inside two ItemLists are not duplicate Products, got %+v", is)
		}
	}
}

// TestIDWithPropertiesIsADeclarationNotAReference pins referencedIDs' bare-stub rule. An object
// carrying @id alongside real properties declares the entity inline; only an object whose sole
// key is @id points elsewhere and can dangle.
func TestIDWithPropertiesIsADeclarationNotAReference(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"WebSite","name":"Shop","publisher":{"@id":"https://shop.test/#org","name":"Shop Inc"}}
	</script></head><body></body></html>`)
	if is, ok := find(structured.New().Analyze(context.Background(), res), "structured-unresolved-id"); ok {
		t.Errorf("an inline declaration with an @id is not a dangling reference, got %+v", is)
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
	// Severity calibration: a malformed price invalidates the Offer and is a Merchant
	// Center disapproval reason, so it belongs at error, not warning.
	if is.Severity != analyze.Error {
		t.Errorf("expected structured-malformed-price at error, got %q", is.Severity)
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
	// Severity calibration: this finding's own explanation says it risks a manual action
	// suppressing every rich result on the site, so it belongs at error, not warning.
	if is.Severity != analyze.Error {
		t.Errorf("expected structured-price-mismatch at error, got %q", is.Severity)
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

func TestNormalizePriceFormatsProduceExactValues(t *testing.T) {
	// Exercises normalizePrice's separator disambiguation for six on-page price formats.
	// Each case is paired against a sentinel markup price (0.01) that cannot coincide with
	// any correct OR plausibly-wrong reading of these page prices, so the assertion cannot
	// pass by accident: if the format fails to parse at all, find() returns ok=false and the
	// test fails outright (rather than the mismatch check being silently guard-suppressed and
	// the test passing for the wrong reason); if it parses to the wrong number, the
	// Data["page"] check catches it directly.
	//
	// An earlier version of this table instead set markup equal to each page price and
	// asserted silence. That let four of the eight rows (plain US decimal, US
	// thousands-then-decimal, comma-only thousands grouping, and — worst of all — dot-only
	// thousands grouping, which fails to parse under the pre-fix code and is silently
	// guard-suppressed rather than actually matched) pass unchanged against the pre-fix,
	// comma-strip-only normalizePrice. Asserting an exact Data["page"] value against a markup
	// that can never coincidentally match removes that blind spot.
	const markup = "0.01"
	cases := []struct {
		name      string
		pagePrice string
		want      string // formatPrice's rendering of the correctly parsed page price
	}{
		{"us decimal point", "$19.99", "19.99"},
		{"eu decimal comma", "€19,99", "19.99"},
		{"us thousands dot then decimal", "$1,299.00", "1299"},
		{"eu thousands dot then decimal comma", "€1.299,00", "1299"},
		{"dot thousands grouping only", "1.234.567 EUR", "1234567"},
		{"comma thousands grouping only", "1,234,567 USD", "1234567"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := page(t, `<html><head><script type="application/ld+json">
				{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
				 "offers":{"@type":"Offer","price":"`+markup+`","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
			</script></head><body><p class="price">`+tc.pagePrice+` </p><button>Add to cart</button></body></html>`)
			// The trailing space inside <p> matters for the "...EUR"/"...USD" suffix
			// cases: goquery's Text() concatenates adjacent elements with no inserted
			// whitespace, so "EUR" immediately followed by "Add" would fail priceRe's
			// trailing \b word boundary (both are word characters) if the space weren't
			// there.
			is, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch")
			if !ok {
				t.Fatalf("page=%s: expected structured-price-mismatch against sentinel markup %s (found none, meaning the page price failed to parse)", tc.pagePrice, markup)
			}
			if is.Data["page"] != tc.want {
				t.Errorf("page=%s: expected normalized page price %s, got %v", tc.pagePrice, tc.want, is.Data["page"])
			}
			if is.Data["markup"] != markup {
				t.Errorf("page=%s: expected markup price %s, got %v", tc.pagePrice, markup, is.Data["markup"])
			}
		})
	}
}

func TestAmbiguousPriceFormsStaySilent(t *testing.T) {
	// A single separator followed by exactly three digits ("$1,299", "€1.299") is genuinely
	// ambiguous: it could be 1299 or 1.299. The markup price (999.00) deliberately differs
	// from every plausible reading of the page price, so a wrongly permissive implementation
	// — one that guesses at the value instead of reporting it unparseable — would produce a
	// mismatch here, not a coincidental match; only correctly recognizing the ambiguity and
	// skipping the comparison keeps this silent.
	cases := []struct {
		name      string
		pagePrice string
	}{
		{"ambiguous single comma", "$1,299"},
		{"ambiguous single dot", "€1.299"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := page(t, `<html><head><script type="application/ld+json">
				{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
				 "offers":{"@type":"Offer","price":"999.00","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
			</script></head><body><p class="price">`+tc.pagePrice+`</p><button>Add to cart</button></body></html>`)
			if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch"); ok {
				t.Errorf("page=%s: an ambiguous price must not be compared against markup", tc.pagePrice)
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
	// were, a page that actually shows two prices (one clear, one ambiguous) would look like
	// it shows exactly one, and the mismatch check would fire on a comparison it has no
	// business making.
	//
	// The markup price (24.99) deliberately DISAGREES with the page's unambiguous price
	// (19.99) — that disagreement is what makes this test load-bearing. Under the correct
	// implementation, distinctPagePrices hits the ambiguous "$1,299", returns nil, and the
	// len(onPage) != 1 guard suppresses the check before markup is even compared: no finding.
	// Under a skip-and-continue implementation that drops unparseable prices instead of
	// giving up on the page, the ambiguous price would simply be dropped, onPage would end up
	// [19.99], the guard would pass, 24.99 would disagree with 19.99, and a finding WOULD
	// fire. An earlier version of this test used a markup price equal to the page's
	// unambiguous price; both the correct and the skip-and-continue implementation produce
	// silence in that shape (agreement in one case, guard-bypass-then-coincidental-agreement
	// in the other), so it never actually exercised the divergence — reverting
	// distinctPagePrices to skip-and-continue left it passing. See the fix-round-2 report
	// entry for the deliberate-revert run that caught this.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"24.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body><p class="price">$19.99</p><s>$1,299</s><button>Add to cart</button></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-price-mismatch"); ok {
		t.Error("an ambiguous price anywhere on the page must suppress the mismatch check, even when a different, unambiguous price on the page would otherwise disagree with markup")
	}
}
