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

//nolint:unused // consumed by later shopify tasks (9-11) that add tests to this package
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
