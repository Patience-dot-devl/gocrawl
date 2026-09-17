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

func page(t *testing.T, html string) *crawler.Result {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return &crawler.Result{Pages: []*crawler.Page{{FinalURL: "https://example.com/", StatusCode: 200, ContentType: "text/html", Doc: doc}}}
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func TestStructuredExtractsTypes(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Organization","name":"Acme"}
	</script></head><body></body></html>`)
	issues := structured.New().Analyze(context.Background(), res)
	is, ok := find(issues, "structured-data")
	if !ok {
		t.Fatal("expected structured-data issue")
	}
	types, _ := is.Data["types"].([]string)
	if len(types) != 1 || types[0] != "Organization" {
		t.Errorf("expected [Organization], got %v", types)
	}
}

func TestStructuredInvalidJSON(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">{ not json }</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-invalid-jsonld"); !ok {
		t.Error("expected invalid-jsonld issue")
	}
}

// TestStructuredInvalidJSONIsError is the severity-calibration fix: a block that fails to
// parse is discarded whole with zero judgement and zero false-positive risk, so it belongs in
// the severity=error set users filter on, not warning.
func TestStructuredInvalidJSONIsError(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">{ not json }</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-invalid-jsonld")
	if !ok {
		t.Fatal("expected invalid-jsonld issue")
	}
	if is.Severity != analyze.Error {
		t.Errorf("expected structured-invalid-jsonld at error, got %q", is.Severity)
	}
}

func TestStructuredGraph(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@graph":[{"@type":"WebSite"},{"@type":"BreadcrumbList"}]}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-data")
	if !ok {
		t.Fatal("expected structured-data issue")
	}
	types, _ := is.Data["types"].([]string)
	if len(types) != 2 {
		t.Errorf("expected 2 types from @graph, got %v", types)
	}
}

func TestStructuredMissingRequired(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Product","image":"x.jpg"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required")
	if !ok {
		t.Fatal("expected structured-missing-required for a Product without name")
	}
	if is.Data["type"] != "Product" {
		t.Errorf("expected type Product in data, got %v", is.Data["type"])
	}
}

func TestStructuredValidProductNoViolation(t *testing.T) {
	// Deliberate change 3: Product's required tier is name, image, offers.price and
	// offers.priceCurrency; offers.availability is recommended (Google documents it as
	// non-blocking), so a complete Product needs the four required fields and availability
	// beside them is just extra completeness, not a requirement.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Product","name":"Widget","image":"https://x.test/w.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Error("did not expect structured-missing-required for a complete Product")
	}
}

func TestStructuredMissingRequiredInGraph(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@graph":[{"@type":"Organization"}]}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); !ok {
		t.Error("expected structured-missing-required for an Organization without name inside @graph")
	}
}

func TestStructuredBreadcrumbCandidate(t *testing.T) {
	res := page(t, `<html><body>
		<nav aria-label="breadcrumb"><a href="/">Home</a> &gt; <a href="/shoes">Shoes</a></nav>
	</body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-breadcrumb-candidate")
	if !ok {
		t.Fatal("expected structured-breadcrumb-candidate")
	}
	if is.Data["links"] != 2 {
		t.Errorf("expected 2 links, got %v", is.Data["links"])
	}
	if is.Data["pages"] != 1 {
		t.Errorf("expected pages 1, got %v", is.Data["pages"])
	}
}

func TestStructuredBreadcrumbCandidateSuppressedByExistingType(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[]}
	</script></head><body>
		<nav aria-label="breadcrumb"><a href="/">Home</a> &gt; <a href="/shoes">Shoes</a></nav>
	</body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-breadcrumb-candidate"); ok {
		t.Error("did not expect structured-breadcrumb-candidate when BreadcrumbList is already present")
	}
}

// breadcrumbPage mirrors dermalogica.nl's collection and product pages: a visible breadcrumb
// trail with the given crumbs and no BreadcrumbList, plus whatever extraHead carries (a
// canonical link, or a JSON-LD block).
func breadcrumbPage(extraHead string, crumbs ...string) string {
	var li strings.Builder
	for _, c := range crumbs {
		li.WriteString(`<li><a href="/` + c + `">` + c + `</a></li>`)
	}
	return `<html><head>` + extraHead + `</head><body>
		<nav class="breadcrumbs" aria-label="breadcrumbs"><ol>` + li.String() + `</ol></nav>
	</body></html>`
}

// TestBreadcrumbCandidateIsSiteScoped pins Fix 4: a theme without BreadcrumbList lacks it on
// every page, so three such pages yield exactly one warning at the site base, not three.
// The page with BreadcrumbList is not counted.
func TestBreadcrumbCandidateIsSiteScoped(t *testing.T) {
	res := pages(t, "https://shop.test", map[string]string{
		"https://shop.test/collections/all":  breadcrumbPage("", "home", "all"),
		"https://shop.test/products/tee":     breadcrumbPage("", "home", "all", "tee"),
		"https://shop.test/products/cap":     breadcrumbPage("", "home", "cap"),
		"https://shop.test/collections/sale": breadcrumbPage(`<script type="application/ld+json">{"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[]}</script>`, "home", "sale"),
	})
	got := findAll(structured.New().Analyze(context.Background(), res), "structured-breadcrumb-candidate")
	if len(got) != 1 {
		t.Fatalf("expected exactly one site-wide breadcrumb candidate, got %d: %+v", len(got), got)
	}
	is := got[0]
	if is.URL != "https://shop.test" {
		t.Errorf("expected the site base URL, got %q", is.URL)
	}
	if is.Severity != analyze.Warning {
		t.Errorf("expected warning, got %v", is.Severity)
	}
	if is.Message != "Pages render breadcrumb navigation but carry no BreadcrumbList structured data" {
		t.Errorf("unexpected message %q", is.Message)
	}
	if is.Data["pages"] != 3 {
		t.Errorf("expected pages 3 (the BreadcrumbList page not counted), got %v", is.Data["pages"])
	}
	if is.Data["links"] != 3 {
		t.Errorf("expected links 3, the largest trail seen, got %v", is.Data["links"])
	}
	if is.Data[analyze.InstanceKey] != "BreadcrumbList" {
		t.Errorf("expected instance BreadcrumbList, got %v", is.Data[analyze.InstanceKey])
	}
	if missing, _ := is.Data["missing"].(map[string]int); missing["BreadcrumbList"] != 3 || len(missing) != 1 {
		t.Errorf("expected missing {BreadcrumbList: 3}, got %v", is.Data["missing"])
	}
	examples, _ := is.Data["examples"].([]string)
	for _, ex := range examples {
		if ex == "https://shop.test/collections/sale" {
			t.Errorf("the BreadcrumbList page must not be an example, got %v", examples)
		}
	}
	if len(examples) != 3 {
		t.Errorf("expected 3 examples, got %v", examples)
	}
}

// TestBreadcrumbCandidateCountsCanonicalOnce pins that the breadcrumb rollup inherits Fix 1's
// dedupe: a product reached at /collections/<c>/products/<h> with a canonical to
// /products/<h> is one page.
func TestBreadcrumbCandidateCountsCanonicalOnce(t *testing.T) {
	res := pages(t, "https://shop.test", map[string]string{
		"https://shop.test/products/tee":                 breadcrumbPage("", "home", "tee"),
		"https://shop.test/collections/all/products/tee": breadcrumbPage(`<link rel="canonical" href="https://shop.test/products/tee">`, "home", "all", "tee"),
	})
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-breadcrumb-candidate")
	if !ok {
		t.Fatal("expected structured-breadcrumb-candidate")
	}
	if is.Data["pages"] != 1 {
		t.Errorf("expected pages 1 for two URLs of one canonical page, got %v", is.Data["pages"])
	}
	if examples, _ := is.Data["examples"].([]string); len(examples) != 1 || examples[0] != "https://shop.test/products/tee" {
		t.Errorf("expected the canonical URL as the only example, got %v", is.Data["examples"])
	}
}

// TestStructuredProductCandidate pins row 1 of the co-location fix: a real product page —
// one product form, with its price inside that same form — still fires. This is a
// regression guard, not a driver: it must pass both before and after the co-location
// rewrite.
func TestStructuredProductCandidate(t *testing.T) {
	res := page(t, `<html><body>
		<h1>Widget</h1>
		<form action="/cart/add" method="post">
			<p class="price">Price: $19.99</p>
			<button type="submit">Add to cart</button>
		</form>
	</body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate")
	if !ok {
		t.Fatal("expected structured-product-candidate")
	}
	if is.Data["signal"] != "$19.99" {
		t.Errorf("expected signal $19.99, got %v", is.Data["signal"])
	}
}

func TestStructuredProductCandidateNoCartSignal(t *testing.T) {
	res := page(t, `<html><body><p>This gadget costs $19.99 to make.</p></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate"); ok {
		t.Error("did not expect structured-product-candidate without a cart/buy signal")
	}
}

// TestStructuredProductCandidateAllbirdsNoProductType pins row 2: the feature's motivating
// example is a real Allbirds PRODUCT page that emits CollectionPage+FAQPage JSON-LD and no
// Product. Type-based suppression (on CollectionPage or ItemList) was rejected specifically
// because it would silence this exact page, so this guard must keep passing regardless of
// what type-shaped changes land later.
func TestStructuredProductCandidateAllbirdsNoProductType(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":["CollectionPage","FAQPage"],"name":"Men's Tree Runners"}
	</script></head><body>
		<h1>Men's Tree Runners</h1>
		<form action="/cart/add" method="post">
			<p class="price">$100.00</p>
			<button type="submit">Add to Cart</button>
		</form>
	</body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate")
	if !ok {
		t.Fatal("expected structured-product-candidate for a product page mislabeled CollectionPage+FAQPage")
	}
	if is.Data["signal"] != "$100.00" {
		t.Errorf("expected signal $100.00, got %v", is.Data["signal"])
	}
}

// TestStructuredProductCandidateSuppressedBySitewideBoilerplate pins row 3: this is the bug.
// A free-shipping-threshold strip near the top of the page supplies a price, and a
// persistent mini-cart drawer supplies "Add to cart", on every page of a real store —
// homepage, policy pages, blog articles included — even though neither is actually part of
// a product. The strip's price and the drawer's button are both only a couple of DOM hops
// under <body>, but they don't share a <form>, so requiring co-location silences this
// without ever reading a schema.org type.
func TestStructuredProductCandidateSuppressedBySitewideBoilerplate(t *testing.T) {
	res := page(t, `<html><body>
		<div class="announcement-bar">Free shipping over $35</div>
		<header><nav><a href="/">Home</a></nav></header>
		<main>
			<h1>About Us</h1>
			<p>We started this company in a garage. It has nothing to do with any single product.</p>
		</main>
		<div id="cart-drawer" class="mini-cart">
			<form action="/cart" method="post">
				<button type="submit">Add to cart</button>
			</form>
		</div>
	</body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate"); ok {
		t.Error("did not expect structured-product-candidate from an unrelated sitewide price and cart-drawer button")
	}
}

// TestStructuredProductCandidateSuppressedOnListingPage pins row 4: a collection page with
// several quick-add tiles has many genuine co-located CTA/price pairs (one per tile), unlike
// a product page's one (or two, for a sticky buy-bar duplicating the main form). Counting
// pairs is what tells this apart from a product page without reading @type — the page below
// deliberately carries no CollectionPage/ItemList markup, to prove the discriminator doesn't
// need it.
func TestStructuredProductCandidateSuppressedOnListingPage(t *testing.T) {
	res := page(t, `<html><body><ul class="product-grid">
		<li><form action="/cart/add"><span class="price">$25.00</span><button>Add to cart</button></form></li>
		<li><form action="/cart/add"><span class="price">$30.00</span><button>Add to cart</button></form></li>
		<li><form action="/cart/add"><span class="price">$40.00</span><button>Add to cart</button></form></li>
		<li><form action="/cart/add"><span class="price">$55.00</span><button>Add to cart</button></form></li>
	</ul></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate"); ok {
		t.Error("did not expect structured-product-candidate on a listing page with many quick-add tiles")
	}
}

// TestStructuredProductCandidateSuppressedByRealProduct pins row 5: a page that actually
// declares Product JSON-LD stays silent even though it also carries a qualifying co-located
// signal, unchanged from before this rewrite.
func TestStructuredProductCandidateSuppressedByRealProduct(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Product","name":"Widget",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD"}}
	</script></head><body>
		<form action="/cart/add"><p class="price">$19.99</p><button>Add to cart</button></form>
	</body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate"); ok {
		t.Error("did not expect structured-product-candidate when Product JSON-LD is already present")
	}
}

// TestStructuredProductCandidateIgnoresHiddenCoLocation guards the co-location fix itself:
// a price and an "Add to cart" button that are structurally co-located (same form) but sit
// inside an aria-hidden="true" subtree — how Shopify marks up a mini-cart drawer before it's
// opened — must not count. Without the hidden-subtree exclusion, an off-screen cart drawer
// populated with real price/CTA pairs would reintroduce the exact false positive this
// rewrite removes, just moved one level down (co-located but never actually shown).
func TestStructuredProductCandidateIgnoresHiddenCoLocation(t *testing.T) {
	res := page(t, `<html><body>
		<main><h1>About Us</h1><p>Nothing product-shaped here.</p></main>
		<div id="cart-drawer" aria-hidden="true">
			<form action="/cart"><span class="price">$35.00</span><button>Add to cart</button></form>
		</div>
	</body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate"); ok {
		t.Error("did not expect structured-product-candidate from a price/CTA pair inside an aria-hidden subtree")
	}
}

func TestStructuredArticleCandidate(t *testing.T) {
	words := strings.Repeat("word ", 150)
	res := page(t, `<html><body><article>
		<h1>Title</h1><time datetime="2026-01-01">Jan 1</time><p>`+words+`</p>
	</article></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-article-candidate")
	if !ok {
		t.Fatal("expected structured-article-candidate")
	}
	if w, _ := is.Data["words"].(int); w < 150 {
		t.Errorf("expected >= 150 words, got %v", is.Data["words"])
	}
}

func TestStructuredArticleCandidateShortArticleIgnored(t *testing.T) {
	res := page(t, `<html><body><article><time datetime="2026-01-01">Jan 1</time><p>Too short.</p></article></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-article-candidate"); ok {
		t.Error("did not expect structured-article-candidate for a short article")
	}
}

func TestStructuredVideoCandidate(t *testing.T) {
	res := page(t, `<html><body>
		<iframe src="https://www.youtube.com/embed/abc123"></iframe>
	</body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-video-candidate")
	if !ok {
		t.Fatal("expected structured-video-candidate")
	}
	if is.Data["src"] != "https://www.youtube.com/embed/abc123" {
		t.Errorf("unexpected src %v", is.Data["src"])
	}
}

func TestStructuredVideoCandidateSuppressedByExistingType(t *testing.T) {
	res := page(t, `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"VideoObject","name":"x","thumbnailUrl":"x.jpg"}
	</script></head><body>
		<iframe src="https://www.youtube.com/embed/abc123"></iframe>
	</body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-video-candidate"); ok {
		t.Error("did not expect structured-video-candidate when VideoObject is already present")
	}
}

// Pinned: the three deliberate behaviour changes from the phase 1 refactor. Each of these
// documents a case whose result intentionally differs from the pre-schemaorg analyzer.

func TestStructuredNestedTypesAreReported(t *testing.T) {
	// Deliberate change 2: the old collectTypes descended only into @graph, so a nested
	// Offer never appeared in the reported type list.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Product","name":"Tee","offers":{"@type":"Offer","price":"19.99",
		 "priceCurrency":"USD","availability":"https://schema.org/InStock"},"image":"https://x.test/a.jpg"}
	</script></head><body></body></html>`)
	is, ok := find(structured.New().Analyze(context.Background(), res), "structured-data")
	if !ok {
		t.Fatal("expected structured-data issue")
	}
	types, _ := is.Data["types"].([]string)
	var sawOffer bool
	for _, ty := range types {
		if ty == "Offer" {
			sawOffer = true
		}
	}
	if !sawOffer {
		t.Errorf("expected the nested Offer in the reported types, got %v", types)
	}
}

func TestStructuredNestedOfferSuppressesProductCandidate(t *testing.T) {
	// Deliberate change 2, second half: a nested Offer now suppresses the product candidate.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"WebPage","mainEntity":{"@type":"Offer","price":"19.99"}}
	</script></head><body><p>$19.99</p><button>Add to cart</button></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-product-candidate"); ok {
		t.Error("did not expect structured-product-candidate when a nested Offer is present")
	}
}

func TestStructuredBareOfferNoLongerRequired(t *testing.T) {
	// Deliberate change 1: Offer left the top-level required-field table.
	res := page(t, `<html><head><script type="application/ld+json">
		{"@type":"Offer","url":"https://x.test/p"}
	</script></head><body></body></html>`)
	if _, ok := find(structured.New().Analyze(context.Background(), res), "structured-missing-required"); ok {
		t.Error("a bare top-level Offer is no longer checked for required fields")
	}
}
