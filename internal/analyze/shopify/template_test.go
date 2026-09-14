package shopify_test

import (
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/shopify"
)

func TestClassify(t *testing.T) {
	cases := map[string]shopify.Template{
		"https://shop.test/":                              shopify.TemplateHome,
		"https://shop.test/products/tee":                  shopify.TemplateProduct,
		"https://shop.test/collections/all/products/tee":  shopify.TemplateProduct,
		"https://shop.test/collections/all":               shopify.TemplateCollection,
		"https://shop.test/collections/all?sort_by=price": shopify.TemplateCollection,
		"https://shop.test/blogs/news/launch":             shopify.TemplateArticle,
		"https://shop.test/blogs/news":                    shopify.TemplateBlog,
		"https://shop.test/pages/about":                   shopify.TemplatePage,
		"https://shop.test/search?q=tee":                  shopify.TemplateUtility,
		"https://shop.test/cart":                          shopify.TemplateUtility,
		"https://shop.test/account/login":                 shopify.TemplateUtility,
		"https://shop.test/apps/reviews":                  shopify.TemplateUnknown,
	}
	for u, want := range cases {
		if got := shopify.Classify(u); got != want {
			t.Errorf("Classify(%q) = %q, want %q", u, got, want)
		}
	}
}
