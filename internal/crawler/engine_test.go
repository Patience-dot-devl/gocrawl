package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home Page Title</title></head><body>
			<a href="/page2">Page 2</a>
			<a href="/missing">Missing</a>
			<a href="/redir">Redirect</a>
			<a href="https://external.example/elsewhere">External</a>
		</body></html>`)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Second Page Here</title></head><body>ok</body></html>`)
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/redir", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/target", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/target", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Redirect Target</title></head><body>arrived</body></html>`)
	})
	return httptest.NewServer(mux)
}

func TestEngineCrawl(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	opts := DefaultOptions()
	opts.MaxDepth = 1
	opts.Concurrency = 2
	opts.UserAgent = "gocrawl-test"
	engine := New(opts, NewHTTPFetcher(opts))

	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("crawl error: %v", err)
	}

	// External host should not be crawled.
	for _, p := range result.Pages {
		if got := p.FinalURL; got != "" && hostOf(got) == "external.example" {
			t.Errorf("external host was crawled: %s", got)
		}
	}

	home, ok := result.Page(ts.URL)
	if !ok {
		t.Fatal("home page not found in result")
	}
	if home.StatusCode != 200 || len(home.Links) < 3 {
		t.Errorf("home: status=%d links=%d", home.StatusCode, len(home.Links))
	}

	missing, ok := result.Page(ts.URL + "/missing")
	if !ok || missing.StatusCode != 404 {
		t.Errorf("missing page: ok=%v status=%d", ok, missing.StatusCode)
	}

	redir, ok := result.Page(ts.URL + "/redir")
	if !ok {
		t.Fatal("redirect page not found")
	}
	if len(redir.Redirects) == 0 {
		t.Error("expected redirect chain to be captured")
	}
	if redir.StatusCode != 200 {
		t.Errorf("expected final status 200 after redirect, got %d", redir.StatusCode)
	}
}

func TestEngineMaxPages(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	opts := DefaultOptions()
	opts.MaxDepth = 5
	opts.MaxPages = 2
	engine := New(opts, NewHTTPFetcher(opts))

	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("crawl error: %v", err)
	}
	if len(result.Pages) > 2 {
		t.Errorf("MaxPages=2 not honored: crawled %d pages", len(result.Pages))
	}
}

func TestEngineCoverageCompleteByDefault(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	// Default options: unlimited depth, bounded only by the (ample) page budget. Every
	// in-scope URL is reachable, so coverage should be complete.
	opts := DefaultOptions()
	if opts.MaxDepth != 0 {
		t.Fatalf("expected default MaxDepth 0 (unlimited), got %d", opts.MaxDepth)
	}
	engine := New(opts, NewHTTPFetcher(opts))
	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("crawl error: %v", err)
	}
	if !result.Coverage.Complete || result.Coverage.DiscoveredNotCrawled != 0 {
		t.Errorf("expected complete coverage, got %+v", result.Coverage)
	}
}

func TestEngineCoveragePartialOnPageLimit(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	opts := DefaultOptions()
	opts.MaxPages = 2
	engine := New(opts, NewHTTPFetcher(opts))
	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("crawl error: %v", err)
	}
	if result.Coverage.Complete {
		t.Error("expected partial coverage when the page limit is hit")
	}
	if !result.Coverage.PageLimitReached {
		t.Error("expected PageLimitReached to be set")
	}
	if result.Coverage.DiscoveredNotCrawled == 0 {
		t.Error("expected DiscoveredNotCrawled > 0")
	}
}

func TestEngineCoveragePartialOnDepthLimit(t *testing.T) {
	// A chain /a -> /b: at MaxDepth 1 we crawl / and /a but decline /b, a real coverage gap.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><body><a href="/a">a</a></body></html>`)
	})
	mux.HandleFunc("/a", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><body><a href="/b">b</a></body></html>`)
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><body>leaf</body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	opts := DefaultOptions()
	opts.MaxDepth = 1
	engine := New(opts, NewHTTPFetcher(opts))
	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("crawl error: %v", err)
	}
	if result.Coverage.Complete {
		t.Error("expected partial coverage when the depth limit hides /b")
	}
	if !result.Coverage.DepthLimitReached {
		t.Error("expected DepthLimitReached to be set")
	}
	if _, ok := result.Page(ts.URL + "/b"); ok {
		t.Error("/b should not have been crawled at MaxDepth 1")
	}
}

func countItemsPages(result *Result) int {
	n := 0
	for _, p := range result.Pages {
		if strings.Contains(p.RequestedURL, "/items") {
			n++
		}
	}
	return n
}

func TestEngineStripQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><body>
			<a href="/items?page=1">1</a>
			<a href="/items?page=2">2</a>
			<a href="/items?page=3">3</a>
		</body></html>`)
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Items List Page</title></head><body>ok</body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	crawl := func(strip bool) *Result {
		opts := DefaultOptions()
		opts.MaxDepth = 1
		opts.StripQuery = strip
		result, err := New(opts, NewHTTPFetcher(opts)).Crawl(context.Background(), ts.URL)
		if err != nil {
			t.Fatalf("crawl error: %v", err)
		}
		return result
	}

	// Without stripping, the three ?page=N links are distinct URLs.
	if got := countItemsPages(crawl(false)); got != 3 {
		t.Errorf("strip-query off: expected 3 /items pages, got %d", got)
	}

	// With stripping, they collapse to a single crawled page...
	stripped := crawl(true)
	if got := countItemsPages(stripped); got != 1 {
		t.Errorf("strip-query on: expected 1 /items page, got %d", got)
	}
	// ...the stored URL carries no query...
	for _, p := range stripped.Pages {
		if strings.Contains(p.RequestedURL, "?") {
			t.Errorf("strip-query on: page URL retained a query string: %q", p.RequestedURL)
		}
	}
	// ...and a lookup by any query variant resolves to that one page.
	if _, ok := stripped.Page(ts.URL + "/items?page=99"); !ok {
		t.Error("strip-query on: lookup by a query variant should resolve to the stripped page")
	}
}

func TestEngineUsesAdaptiveLimiter(t *testing.T) {
	opts := DefaultOptions()
	opts.AdaptiveDelay = true
	engine := New(opts, NewHTTPFetcher(opts))

	page := &Page{StatusCode: http.StatusTooManyRequests, FinalURL: "https://example.test/"}
	adjusted, next := engine.limiter.OnResponse(page.StatusCode, page.Header)
	if !adjusted || next != backoffStartRate {
		t.Fatalf("first 429: adjusted=%v next=%v, want true/%v", adjusted, next, backoffStartRate)
	}
}

func hostOf(raw string) string {
	const p = "://"
	i := indexOf(raw, p)
	if i < 0 {
		return ""
	}
	rest := raw[i+len(p):]
	for j := 0; j < len(rest); j++ {
		if rest[j] == '/' {
			return rest[:j]
		}
	}
	return rest
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
