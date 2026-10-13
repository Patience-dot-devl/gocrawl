package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestEngineInScopeBeforeCrawl(t *testing.T) {
	opts := DefaultOptions()
	opts.Exclude = []*regexp.Regexp{regexp.MustCompile(`/private`)}
	e := New(opts, NewHTTPFetcher(opts))
	seed, err := e.SetSeed("HTTPS://Example.com:443/")
	if err != nil {
		t.Fatalf("SetSeed: %v", err)
	}
	if seed != "https://example.com/" {
		t.Fatalf("SetSeed returned %q, want the normalized https://example.com/", seed)
	}
	cases := map[string]bool{
		"https://example.com/a":         true,
		"https://EXAMPLE.com/b/":        true,
		"https://shop.example.com/a":    false,
		"https://other.org/":            false,
		"https://example.com/private/x": false,
		"mailto:hi@example.com":         false,
	}
	for u, want := range cases {
		if got := e.InScope(u); got != want {
			t.Errorf("InScope(%q) = %v, want %v", u, got, want)
		}
	}

	opts.AllowSubdomains = true
	sub := New(opts, NewHTTPFetcher(opts))
	if _, err := sub.SetSeed("https://example.com/"); err != nil {
		t.Fatalf("SetSeed: %v", err)
	}
	if !sub.InScope("https://shop.example.com/a") {
		t.Error("with AllowSubdomains, a subdomain URL should be in scope")
	}
}

func TestEngineNormalize(t *testing.T) {
	opts := DefaultOptions()
	e := New(opts, NewHTTPFetcher(opts))
	if got := e.Normalize("https://Example.com/a/#top"); got != "https://example.com/a" {
		t.Errorf("Normalize = %q, want https://example.com/a", got)
	}
	opts.StripQuery = true
	strip := New(opts, NewHTTPFetcher(opts))
	if got := strip.Normalize("https://example.com/a?page=2"); got != "https://example.com/a" {
		t.Errorf("Normalize with StripQuery = %q, want https://example.com/a", got)
	}
}

func TestEngineRobotsForReadsSitemapLines(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nAllow: /\nSitemap: https://cdn.example/sitemap.xml\n")
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	opts := DefaultOptions()
	e := New(opts, NewHTTPFetcher(opts))
	rd := e.RobotsFor(context.Background(), ts.URL+"/")
	if rd == nil || !rd.Found {
		t.Fatalf("RobotsFor = %+v, want a found robots.txt", rd)
	}
	if len(rd.Sitemaps) != 1 || rd.Sitemaps[0] != "https://cdn.example/sitemap.xml" {
		t.Errorf("Sitemaps = %v, want [https://cdn.example/sitemap.xml]", rd.Sitemaps)
	}
	if got := e.RobotsFor(context.Background(), "::bad"); got != nil {
		t.Errorf("RobotsFor on an unparseable URL = %+v, want nil", got)
	}
}
