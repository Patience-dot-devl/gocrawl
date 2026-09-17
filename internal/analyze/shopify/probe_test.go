package shopify_test

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/shopify"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// stubFetcher answers a fixed set of URLs and records which were requested, so a test can
// assert that the probe stays off unless it was asked for.
type stubFetcher struct {
	pages     map[string]*crawler.Page
	requested []string
}

func (f *stubFetcher) Fetch(_ context.Context, rawURL string) (*crawler.Page, error) {
	f.requested = append(f.requested, rawURL)
	if p, ok := f.pages[rawURL]; ok {
		return p, nil
	}
	return &crawler.Page{RequestedURL: rawURL, FinalURL: rawURL, StatusCode: 404}, nil
}

func TestProductsJSONProbeOffByDefault(t *testing.T) {
	f := &stubFetcher{pages: map[string]*crawler.Page{}}
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/": shopifyHome})
	shopify.New(f).Analyze(context.Background(), res)
	if len(f.requested) != 0 {
		t.Errorf("the analyzer must make no requests unless probes are enabled, got %v", f.requested)
	}
}

func TestProductsJSONProbeReportsAnExposedFeed(t *testing.T) {
	body := `{"products":[{"id":1,"title":"Tee","handle":"tee","variants":[{"id":11,"price":"19.99"}]}]}`
	f := &stubFetcher{pages: map[string]*crawler.Page{
		"https://shop.test/products.json?limit=1": {
			FinalURL: "https://shop.test/products.json?limit=1", StatusCode: 200,
			ContentType: "application/json", Body: []byte(body),
		},
	}}
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/": shopifyHome})
	issues := shopify.New(f, shopify.WithProbes(true)).Analyze(context.Background(), res)
	is, ok := find(issues, "shopify-products-json-exposed")
	if !ok {
		t.Fatal("expected shopify-products-json-exposed")
	}
	if is.URL != "https://shop.test/products.json" {
		t.Errorf("expected the feed URL without the probe's limit, got %q", is.URL)
	}
}

func TestProductsJSONProbeSilentWhenClosed(t *testing.T) {
	f := &stubFetcher{pages: map[string]*crawler.Page{}} // every URL 404s
	res := store(t, "https://shop.test", map[string]string{"https://shop.test/": shopifyHome})
	issues := shopify.New(f, shopify.WithProbes(true)).Analyze(context.Background(), res)
	if _, ok := find(issues, "shopify-products-json-exposed"); ok {
		t.Error("a 404 feed is not exposed")
	}
	if _, ok := find(issues, "shopify-detected"); !ok {
		t.Fatal("fixture must be detected as Shopify, otherwise this test passes vacuously")
	}
}
