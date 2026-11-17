package crawler

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

// FuzzResolveURL feeds hostile hrefs, as found in crawled pages, through link resolution.
// Beyond not panicking, an accepted link must be an absolute http(s) URL, and its dedup key
// must be stable: normalizing it again changes nothing, or the crawl index would hold the same
// page under two keys.
func FuzzResolveURL(f *testing.F) {
	for _, s := range []string{
		"/a/b/", "../x?q=1#frag", "https://Example.COM:443/P/", "//cdn.example.com/x",
		"mailto:a@b.c", "javascript:void(0)", "#top", " http://[::1]:80/ ", "?a=1&b=2",
		"http://user:pass@example.com/", "%zz", "http://exa mple.com/",
	} {
		f.Add(s, false)
		f.Add(s, true)
	}
	base, _ := url.Parse("https://example.com/dir/page")
	f.Fuzz(func(t *testing.T, href string, stripQuery bool) {
		key, resolved, ok := resolveURL(base, href, stripQuery)
		if !ok {
			if key != "" || resolved != "" {
				t.Fatalf("rejected href %q returned key=%q resolved=%q", href, key, resolved)
			}
			return
		}
		for _, s := range []string{key, resolved} {
			u, err := url.Parse(s)
			if err != nil {
				t.Fatalf("href %q resolved to unparsable %q: %v", href, s, err)
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				t.Fatalf("href %q resolved to non-http(s) %q", href, s)
			}
		}
		if again := normalizeURL(key, stripQuery); again != key {
			t.Fatalf("dedup key not stable for href %q: %q then %q", href, key, again)
		}
	})
}

// FuzzRobots runs arbitrary robots.txt bodies through the robots manager and asks it about
// arbitrary paths. A malformed file on a crawled site must never crash the crawl.
func FuzzRobots(f *testing.F) {
	f.Add("User-agent: *\nDisallow: /private\nCrawl-delay: 5\nSitemap: https://example.com/s.xml\n", "/private/x")
	f.Add("User-agent: gocrawl\nAllow: /$\nDisallow: /*?\n", "/?a=1")
	f.Add("\xef\xbb\xbfUser-agent:*\r\nDisallow:\r\n", "/")
	f.Add("Crawl-delay: -1e309\nUser-agent", "")
	f.Fuzz(func(t *testing.T, body, path string) {
		fetch := fakeFetcherFunc(func(_ context.Context, rawURL string) (*Page, error) {
			return &Page{FinalURL: rawURL, StatusCode: 200, Body: []byte(body)}, nil
		})
		m := newRobotsManager(fetch, "gocrawl")
		u, err := url.Parse("https://example.com/" + strings.TrimPrefix(path, "/"))
		if err != nil {
			return
		}
		_ = m.allowed(context.Background(), u)
		_ = m.get(context.Background(), u)
	})
}
