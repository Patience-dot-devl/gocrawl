package structured

import "testing"

// TestEveryRankedTypeHasAWeight keeps a newly added eligibility type, or a candidate type,
// from silently scoring zero in the opportunity ranking.
func TestEveryRankedTypeHasAWeight(t *testing.T) {
	types := []string{"Product", "Article", "VideoObject", "BreadcrumbList"} // candidate checks
	for ty := range eligibility {
		types = append(types, ty)
	}
	for _, ty := range types {
		if typeWeights[ty] <= 0 {
			t.Errorf("type %s has no type_weight", ty)
		}
	}
}

func TestSection(t *testing.T) {
	for in, want := range map[string]string{
		"https://shop.test":                         "/",
		"https://shop.test/":                        "/",
		"https://shop.test/?q=1":                    "/",
		"https://shop.test/my-post":                 "/*",
		"https://shop.test/my-post/":                "/*",
		"https://shop.test/products/tee":            "/products/*",
		"https://shop.test/blogs/news/launch":       "/blogs/*",
		"https://shop.test/fr/products/tee#reviews": "/fr/*",
	} {
		if got := section(in); got != want {
			t.Errorf("section(%q) = %q, want %q", in, got, want)
		}
	}
}
