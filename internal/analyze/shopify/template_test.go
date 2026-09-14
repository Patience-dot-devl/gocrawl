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

		// Shopify Markets locale/region prefixes must not swallow the whole store into
		// TemplateUnknown.
		"https://shop.test/en-ca/products/tee":      shopify.TemplateProduct,
		"https://shop.test/fr/collections/all":      shopify.TemplateCollection,
		"https://shop.test/de-de/blogs/news/launch": shopify.TemplateArticle,
		"https://shop.test/en-ca/":                  shopify.TemplateHome,

		// /blogs/<handle>/tagged/<tag> is a filtered listing, not a single post.
		"https://shop.test/blogs/news/tagged/summer": shopify.TemplateBlog,

		// Policy pages are meant to be indexed, so they get their own template rather than
		// folding into TemplateUtility; /password belongs with the other utility gates.
		"https://shop.test/policies/refund-policy": shopify.TemplatePolicy,
		"https://shop.test/password":               shopify.TemplateUtility,
	}
	for u, want := range cases {
		if got := shopify.Classify(u); got != want {
			t.Errorf("Classify(%q) = %q, want %q", u, got, want)
		}
	}
}
