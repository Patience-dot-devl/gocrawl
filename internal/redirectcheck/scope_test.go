package redirectcheck_test

import (
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func rule(original, target string) redirectcheck.Rule {
	return redirectcheck.Rule{Original: original, Target: target}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		r      redirectcheck.Rule
		domain string
		want   redirectcheck.Scope
	}{
		{"relative target on main domain", rule("/old", "/new"), "example.com", redirectcheck.ScopeInScope},
		{"absolute target on main domain", rule("/old", "https://example.com/new"), "example.com", redirectcheck.ScopeInScope},
		{"absolute target on subdomain", rule("/old", "https://shop.example.com/new"), "example.com", redirectcheck.ScopeInScope},
		{"absolute target on external domain", rule("/old", "https://other-site.com/new"), "example.com", redirectcheck.ScopeExternal},
		{"regex original is dynamic", rule("https?://example.com/cases(?P<page_slug>/.*)?$", "https://example.com/en/cases{page_slug}"), "example.com", redirectcheck.ScopeDynamic},
		{"placeholder target is dynamic", rule("/cases", "https://example.com/en/cases{page_slug}"), "example.com", redirectcheck.ScopeDynamic},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := redirectcheck.Classify(c.r, c.domain)
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			if got != c.want {
				t.Errorf("Classify(%+v, %q) = %q, want %q", c.r, c.domain, got, c.want)
			}
		})
	}
}
