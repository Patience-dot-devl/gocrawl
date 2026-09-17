package shopify_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/shopify"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// store builds a Result from (url, html) pairs. Body is populated as well as Doc because the
// analyzer fingerprints the raw HTML, not only the parsed DOM.
func store(t *testing.T, seed string, urlHTML map[string]string) *crawler.Result {
	t.Helper()
	res := &crawler.Result{Seed: seed}
	for u, html := range urlHTML {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatalf("parse %s: %v", u, err)
		}
		res.Pages = append(res.Pages, &crawler.Page{
			FinalURL: u, StatusCode: 200, ContentType: "text/html",
			Body: []byte(html), Doc: doc,
		})
	}
	return res
}

func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

func findAll(issues []analyze.Issue, code string) []analyze.Issue {
	var out []analyze.Issue
	for _, is := range issues {
		if is.Code == code {
			out = append(out, is)
		}
	}
	return out
}

func run(t *testing.T, res *crawler.Result) []analyze.Issue {
	t.Helper()
	return shopify.New(nil).Analyze(context.Background(), res)
}

const shopifyHome = `<html><head>
	<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
	<script>var Shopify = Shopify || {}; Shopify.theme = {"name":"Dawn","id":123456,"theme_store_id":887,"role":"main"};</script>
</head><body><h1>Shop</h1></body></html>`

func TestDetectsShopify(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/": shopifyHome})
	is, ok := find(run(t, res), "shopify-detected")
	if !ok {
		t.Fatal("expected shopify-detected")
	}
	if is.URL != "https://shop.test" {
		t.Errorf("expected the site base as the issue URL, got %q", is.URL)
	}
	if is.Data["theme"] != "Dawn" {
		t.Errorf("expected theme Dawn, got %v", is.Data["theme"])
	}
	if is.Severity != analyze.Info {
		t.Errorf("expected info severity, got %q", is.Severity)
	}
}

func TestDetectsShopifyFromHeaderAlone(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/": `<html><body>Hi</body></html>`})
	res.Pages[0].Header = map[string][]string{"X-Shopid": {"123456"}}
	if _, ok := find(run(t, res), "shopify-detected"); !ok {
		t.Error("expected the X-ShopId header alone to identify a Shopify store")
	}
}

func TestSilentOnNonShopifySite(t *testing.T) {
	res := store(t, "https://blog.test", map[string]string{
		"https://blog.test/":           `<html><head><meta name="generator" content="Hugo"></head><body>Hi</body></html>`,
		"https://blog.test/products/a": `<html><body><h1>A thing</h1><p>$19.99</p></body></html>`,
	})
	if issues := run(t, res); len(issues) != 0 {
		t.Errorf("expected complete silence on a non-Shopify site, got %d issues: %+v", len(issues), issues)
	}
}

// TestSilentOnWeakMarkersAlone reproduces the huel.com / Shopify Buy Button shape: a site that
// is NOT itself a Shopify storefront but embeds a Shopify Buy Button widget, so its page loads
// cdn.shopify.com assets and references a *.myshopify.com backing store without carrying either
// strong theme-runtime fingerprint anywhere on the site. Weak markers alone must not flip
// detection, or every such embed gets misclassified as a full storefront.
func TestSilentOnWeakMarkersAlone(t *testing.T) {
	res := store(t, "https://blog.test", map[string]string{
		"https://blog.test/": `<html><head>
			<script src="https://cdn.shopify.com/s/files/1/buy-button/buy-button-storefront.js"></script>
			<script>var buyButtonShop = "example-store.myshopify.com";</script>
		</head><body><h1>Buy our thing</h1></body></html>`,
	})
	if issues := run(t, res); len(issues) != 0 {
		t.Errorf("expected complete silence on weak markers alone (Buy Button embed), got %d issues: %+v", len(issues), issues)
	}
}

const bareProductPage = `<html><head>
	<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
</head><body><h1>Tee</h1><p>$19.99</p><button>Add to cart</button></body></html>`

func TestTemplateSchemaGapRollsUpPerTemplate(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":           shopifyHome,
		"https://shop.test/products/a": bareProductPage,
		"https://shop.test/products/b": bareProductPage,
		"https://shop.test/products/c": bareProductPage,
	})
	gaps := findAll(run(t, res), "shopify-template-schema-gap")
	var product []map[string]any
	for _, is := range gaps {
		if is.Data["template"] == "product" {
			product = append(product, is.Data)
		}
	}
	// One gap for the missing Product group, one for the missing BreadcrumbList group.
	if len(product) != 2 {
		t.Fatalf("expected 2 product-template gaps, got %d (%+v)", len(product), gaps)
	}
	for _, d := range product {
		if d["pages"] != 3 {
			t.Errorf("expected pages=3, got %v", d["pages"])
		}
		if len(d["examples"].([]string)) != 3 {
			t.Errorf("expected 3 examples, got %v", d["examples"])
		}
	}
	// Every gap shares code and URL, so each needs its own instance or gocrawl compare collapses
	// them into one finding.
	instances := map[any]bool{}
	for _, is := range gaps {
		inst, _ := is.Data[analyze.InstanceKey].(string)
		if inst == "" || instances[inst] {
			t.Errorf("expected a distinct non-empty instance per gap, got %q", inst)
		}
		instances[inst] = true
	}
	for _, is := range gaps {
		if is.Data["template"] == "product" && !strings.Contains(is.Message, "3 Shopify product pages have no") {
			t.Errorf("expected the message to carry the affected-page count, got %q", is.Message)
		}
	}
}

// TestTemplateSchemaGapMessageSingular pins the singular wording: a bare "1 ... pages have no
// ..." message would read as a grammar bug and, worse, would look like the blanket-claim
// overclaim this message was fixed to avoid.
func TestTemplateSchemaGapMessageSingular(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":           shopifyHome,
		"https://shop.test/products/a": bareProductPage,
	})
	gaps := findAll(run(t, res), "shopify-template-schema-gap")
	var found bool
	for _, is := range gaps {
		if is.Data["template"] != "product" {
			continue
		}
		found = true
		if !strings.Contains(is.Message, "1 Shopify product page has no") {
			t.Errorf("expected singular wording, got %q", is.Message)
		}
	}
	if !found {
		t.Fatal("expected a product-template gap")
	}
}

func TestTemplateSchemaGapSilentWhenSchemaPresent(t *testing.T) {
	product := `<html><head>
		<script src="https://cdn.shopify.com/s/files/1/0/assets/theme.js"></script>
		<script>Shopify.theme = {"name":"Dawn","id":123456};</script>
		<script type="application/ld+json">{"@type":"Product","name":"Tee","image":"https://shop.test/t.jpg",
		 "offers":{"@type":"Offer","price":"19.99","priceCurrency":"USD","availability":"https://schema.org/InStock"}}</script>
		<script type="application/ld+json">{"@type":"BreadcrumbList","itemListElement":[]}</script>
	</head><body><h1>Tee</h1></body></html>`
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/products/a": product})
	issues := run(t, res)
	// Guard against the vacuous pass: if detection failed, Analyze returns nil and this test
	// would pass for the wrong reason. Confirm the analyzer actually recognized the store.
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("expected shopify-detected, otherwise this test passes vacuously")
	}
	for _, is := range findAll(issues, "shopify-template-schema-gap") {
		if is.Data["template"] == "product" {
			t.Errorf("unexpected product gap: %+v", is.Data)
		}
	}
}

func TestUtilityTemplateExpectsNothing(t *testing.T) {
	res := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":     shopifyHome,
		"https://shop.test/cart": bareProductPage,
	})
	for _, is := range findAll(run(t, res), "shopify-template-schema-gap") {
		if is.Data["template"] == "utility" {
			t.Error("utility pages should have no schema expectation")
		}
	}
}

// TestTemplateSchemaGapLocalizedStoreSameAsUnlocalized is the test that actually proves the
// Markets-locale silence is gone: a TestClassify row alone only pins one URL's classification,
// not that a whole crawled store keeps reporting gaps once every URL carries a locale prefix.
func TestTemplateSchemaGapLocalizedStoreSameAsUnlocalized(t *testing.T) {
	localized := store(t, "https://shop.test", map[string]string{
		"https://shop.test/en-ca/":           shopifyHome,
		"https://shop.test/en-ca/products/a": bareProductPage,
		"https://shop.test/en-ca/products/b": bareProductPage,
	})
	plain := store(t, "https://shop.test", map[string]string{
		"https://shop.test/":           shopifyHome,
		"https://shop.test/products/a": bareProductPage,
		"https://shop.test/products/b": bareProductPage,
	})

	productGaps := func(res *crawler.Result) []map[string]any {
		var out []map[string]any
		for _, is := range findAll(run(t, res), "shopify-template-schema-gap") {
			if is.Data["template"] == "product" {
				out = append(out, is.Data)
			}
		}
		return out
	}

	localizedProduct := productGaps(localized)
	plainProduct := productGaps(plain)

	if len(localizedProduct) == 0 {
		t.Fatal("expected product gaps on the localized (/en-ca/) store; got none — a Markets " +
			"locale prefix silenced classification for the whole store")
	}
	if len(localizedProduct) != len(plainProduct) {
		t.Fatalf("localized store produced %d product gaps, unlocalized produced %d; a Markets "+
			"storefront must report the same gaps as its unlocalized equivalent", len(localizedProduct), len(plainProduct))
	}
	for i := range localizedProduct {
		if localizedProduct[i]["pages"] != plainProduct[i]["pages"] {
			t.Errorf("gap %d: localized pages=%v, plain pages=%v", i, localizedProduct[i]["pages"], plainProduct[i]["pages"])
		}
	}
}
