package analyze

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

type stub struct{ name string }

func (s stub) Name() string        { return s.name }
func (s stub) Description() string { return "stub " + s.name }
func (s stub) Analyze(_ context.Context, _ *crawler.Result) []Issue {
	return []Issue{{Analyzer: s.name, Code: s.name + "-hit"}}
}

func names(as []Analyzer) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name())
	}
	return out
}

func newReg(ns ...string) *Registry {
	r := NewRegistry()
	for _, n := range ns {
		r.Register(stub{n})
	}
	return r
}

func TestRegistryKeepsRegistrationOrder(t *testing.T) {
	r := newReg("seo", "links", "robots")
	if got := r.Names(); !reflect.DeepEqual(got, []string{"seo", "links", "robots"}) {
		t.Fatalf("Names() = %v", got)
	}
	if got := names(r.All()); !reflect.DeepEqual(got, []string{"seo", "links", "robots"}) {
		t.Fatalf("All() = %v", got)
	}
}

func TestRegistryReRegisterReplacesInPlace(t *testing.T) {
	r := newReg("seo", "links")
	r.Register(stub{"seo"})
	if got := r.Names(); !reflect.DeepEqual(got, []string{"seo", "links"}) {
		t.Fatalf("Names() after re-register = %v, want original order without duplicates", got)
	}
	if _, ok := r.Get("seo"); !ok {
		t.Fatal("Get(seo) missing after re-register")
	}
	if _, ok := r.Get("nope"); ok {
		t.Fatal("Get(nope) found an unregistered analyzer")
	}
}

func TestRegistrySelect(t *testing.T) {
	r := newReg("seo", "links", "robots")
	cases := []struct {
		name              string
		enabled, disabled []string
		want              []string
	}{
		{"all by default", nil, nil, []string{"seo", "links", "robots"}},
		{"disabled removes", nil, []string{"links"}, []string{"seo", "robots"}},
		{"enabled restricts in registration order", []string{"robots", "seo"}, nil, []string{"seo", "robots"}},
		{"disabled wins over enabled", []string{"seo", "links"}, []string{"links"}, []string{"seo"}},
		{"unknown enabled ignored", []string{"nope"}, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := names(r.Select(tc.enabled, tc.disabled))
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Select(%v, %v) = %v, want %v", tc.enabled, tc.disabled, got, tc.want)
			}
		})
	}
}

func TestRunConcatenatesInOrder(t *testing.T) {
	r := newReg("a", "b")
	issues := Run(context.Background(), r.All(), &crawler.Result{})
	got := []string{issues[0].Code, issues[1].Code}
	if !reflect.DeepEqual(got, []string{"a-hit", "b-hit"}) {
		t.Fatalf("Run codes = %v", got)
	}
}

func TestEachPageVisitsEveryPage(t *testing.T) {
	result := &crawler.Result{Pages: []*crawler.Page{{FinalURL: "a"}, {FinalURL: "b"}}}
	issues := EachPage(result, func(p *crawler.Page) []Issue {
		return []Issue{{URL: p.FinalURL}}
	})
	if len(issues) != 2 || issues[0].URL != "a" || issues[1].URL != "b" {
		t.Fatalf("EachPage issues = %+v", issues)
	}
}

func TestSiteBase(t *testing.T) {
	if got := SiteBase(&crawler.Result{Seed: "https://shop.example.com/collections/all?x=1"}); got != "https://shop.example.com" {
		t.Errorf("from seed = %q", got)
	}
	r := &crawler.Result{Seed: "not a url", Pages: []*crawler.Page{{FinalURL: "http://example.org/p"}}}
	if got := SiteBase(r); got != "http://example.org" {
		t.Errorf("from first page = %q", got)
	}
	if got := SiteBase(&crawler.Result{Seed: "garbage"}); got != "garbage" {
		t.Errorf("fallback = %q", got)
	}
}

func page(t *testing.T, finalURL, html string) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	return &crawler.Page{FinalURL: finalURL, Doc: doc}
}

func TestDeclaredCanonicalResolvesRelativeAndIgnoresBody(t *testing.T) {
	p := page(t, "https://example.com/products/tee?variant=1",
		`<html><head><link rel="canonical" href="/products/tee"></head><body></body></html>`)
	if got := DeclaredCanonical(p); got != "https://example.com/products/tee" {
		t.Errorf("relative canonical = %q", got)
	}
	body := page(t, "https://example.com/x", `<html><head></head><body><link rel="canonical" href="/y"></body></html>`)
	if got := DeclaredCanonical(body); got != "" {
		t.Errorf("body canonical = %q, want ignored", got)
	}
	if got := DeclaredCanonical(&crawler.Page{FinalURL: "https://example.com/"}); got != "" {
		t.Errorf("nil Doc = %q, want empty", got)
	}
}

func TestCanonicalURLFallsBackToFinalURL(t *testing.T) {
	p := page(t, "https://example.com/plain", `<html><head></head><body></body></html>`)
	if got := CanonicalURL(p); got != "https://example.com/plain" {
		t.Errorf("CanonicalURL = %q", got)
	}
}

func TestURLKey(t *testing.T) {
	cases := map[string]string{
		"https://example.com/a/":     "https://example.com/a",
		"https://example.com/a#frag": "https://example.com/a",
		"https://example.com/a/#x":   "https://example.com/a",
		"https://example.com/a?q=1":  "https://example.com/a?q=1",
	}
	for in, want := range cases {
		if got := URLKey(in); got != want {
			t.Errorf("URLKey(%q) = %q, want %q", in, got, want)
		}
	}
}
