package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// TestEngineDoesNotDoubleReportRedirectTarget guards against a real bug: a redirect's
// destination was never marked visited, so a separate direct link to that same URL produced a
// second *Page entry for identical content, and every per-page analyzer would report its
// issues twice. Both links here are discovered on the same page and enqueued back-to-back, so
// the redirect ("/old") and the direct link ("/new") are always in flight concurrently — this
// is the race the fix can't eliminate (both requests reach the server), but it must still
// collapse to exactly one recorded page.
func TestEngineDoesNotDoubleReportRedirectTarget(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home</title></head><body>
			<a href="/old">Old link</a>
			<a href="/new">Direct link</a>
		</body></html>`)
	})
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>New Page</title></head><body>ok</body></html>`)
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

	newPages := 0
	for _, p := range result.Pages {
		if p.FinalURL == ts.URL+"/new" {
			newPages++
		}
	}
	if newPages != 1 {
		t.Errorf("result.Pages has %d entries for /new, want 1 (duplicate content would double-report every analyzer issue)", newPages)
	}
}

// TestEngineSkipsSequentialDuplicateFetch is the non-racing case the fix fully closes: the
// redirect resolves (and its destination is marked visited) before a separate page's link to
// that same destination is ever discovered, so the second fetch never even starts.
func TestEngineSkipsSequentialDuplicateFetch(t *testing.T) {
	var newHits int
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home</title></head><body><a href="/old">Old link</a></body></html>`)
	})
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, _ *http.Request) {
		newHits++
		fmt.Fprint(w, `<html><head><title>New Page</title></head><body><a href="/also-links-here">Another</a></body></html>`)
	})
	mux.HandleFunc("/also-links-here", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Also Links Here</title></head><body><a href="/new">Direct link, discovered later</a></body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	opts := DefaultOptions()
	opts.MaxDepth = 3    // deep enough that the direct /new link (discovered 2 hops later) is still enqueued
	opts.Concurrency = 1 // force strictly sequential fetches so the race can't occur
	engine := New(opts, NewHTTPFetcher(opts))
	if _, err := engine.Crawl(context.Background(), ts.URL); err != nil {
		t.Fatalf("crawl error: %v", err)
	}

	if newHits != 1 {
		t.Errorf("/new was fetched %d times, want 1", newHits)
	}
}

func extractLinksFromHTML(t *testing.T, finalURL, html string) []Link {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	page := &Page{FinalURL: finalURL, Doc: doc}
	opts := DefaultOptions()
	e := New(opts, NewHTTPFetcher(opts))
	u, err := url.Parse(finalURL)
	if err != nil {
		t.Fatalf("parse finalURL: %v", err)
	}
	e.seedHost = u.Host
	return e.extractLinks(page)
}

// TestEngineWiresAllowRedirectOnHTTPFetcher guards against a real SSRF/scope-escape surface:
// New must wire the HTTPFetcher's allowRedirect to the engine's own scope check, so a redirect
// mid-fetch is gated exactly like an ordinary link, rather than escaping scope unchecked (a
// relevant concern when gocrawl runs as an MCP server). RespectRobots is disabled here so the
// in-scope case doesn't perform a real network fetch of robots.txt; the robots half of the
// check is covered separately by TestRobotsManagerFetch and TestFetchBlocksRedirectRejectedByAllowRedirect.
func TestEngineWiresAllowRedirectOnHTTPFetcher(t *testing.T) {
	opts := DefaultOptions()
	opts.RespectRobots = false
	hf := NewHTTPFetcher(opts)
	e := New(opts, hf)
	if hf.allowRedirect == nil {
		t.Fatal("expected Engine.New to wire allowRedirect onto the HTTPFetcher")
	}
	e.seedHost = "example.com"

	inScope, err := url.Parse("https://example.com/other")
	if err != nil {
		t.Fatal(err)
	}
	outOfScope, err := url.Parse("https://evil.example/other")
	if err != nil {
		t.Fatal(err)
	}
	if !hf.allowRedirect(context.Background(), inScope) {
		t.Error("expected an in-scope redirect target to be allowed")
	}
	if hf.allowRedirect(context.Background(), outOfScope) {
		t.Error("expected an out-of-scope redirect target to be blocked")
	}
}

// TestEngineRobotsUsesRotationPoolUserAgent guards against a real bug: robots.txt checks used
// opts.UserAgent even when a UserAgents rotation pool superseded it for actual requests, so
// the crawl could test the wrong identity against a per-agent robots.txt rule.
func TestEngineRobotsUsesRotationPoolUserAgent(t *testing.T) {
	opts := DefaultOptions()
	opts.UserAgent = "gocrawl-default"
	opts.UserAgents = []string{"BotA", "BotB"}
	e := New(opts, NewHTTPFetcher(opts))
	if e.robots.userAgent != "BotA" {
		t.Errorf("robots.userAgent = %q, want %q (first entry of the rotation pool)", e.robots.userAgent, "BotA")
	}
}

func TestExtractLinksHonorsBaseHref(t *testing.T) {
	links := extractLinksFromHTML(t, "https://example.com/dir/page",
		`<html><head><base href="https://other.example/sub/"></head>
		<body><a href="page2">Page 2</a></body></html>`)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if want := "https://other.example/sub/page2"; links[0].URL != want {
		t.Errorf("link URL = %q, want %q", links[0].URL, want)
	}
	if !links[0].External {
		t.Error("expected link resolved via a cross-host <base> to be flagged External")
	}
}

func TestExtractLinksRelativeBaseHref(t *testing.T) {
	links := extractLinksFromHTML(t, "https://example.com/dir/page",
		`<html><head><base href="/newroot/"></head>
		<body><a href="x">X</a></body></html>`)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if want := "https://example.com/newroot/x"; links[0].URL != want {
		t.Errorf("link URL = %q, want %q", links[0].URL, want)
	}
}

func TestExtractLinksOnlyFirstBaseCounts(t *testing.T) {
	links := extractLinksFromHTML(t, "https://example.com/",
		`<html><head><base href="/first/"><base href="/second/"></head>
		<body><a href="x">X</a></body></html>`)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if want := "https://example.com/first/x"; links[0].URL != want {
		t.Errorf("link URL = %q, want %q (only the first <base> should count)", links[0].URL, want)
	}
}

func TestExtractLinksBaseWithoutHrefIsSkipped(t *testing.T) {
	links := extractLinksFromHTML(t, "https://example.com/dir/page",
		`<html><head><base target="_blank"></head>
		<body><a href="x">X</a></body></html>`)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if want := "https://example.com/dir/x"; links[0].URL != want {
		t.Errorf("link URL = %q, want %q", links[0].URL, want)
	}
}

func TestExtractLinksEmptyBaseHrefFallsBack(t *testing.T) {
	links := extractLinksFromHTML(t, "https://example.com/dir/page",
		`<html><head><base href=""></head>
		<body><a href="x">X</a></body></html>`)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if want := "https://example.com/dir/x"; links[0].URL != want {
		t.Errorf("link URL = %q, want %q", links[0].URL, want)
	}
}

func TestExtractLinksNoBaseUnchanged(t *testing.T) {
	links := extractLinksFromHTML(t, "https://example.com/dir/page",
		`<html><head></head><body><a href="x">X</a></body></html>`)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if want := "https://example.com/dir/x"; links[0].URL != want {
		t.Errorf("link URL = %q, want %q", links[0].URL, want)
	}
}

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

// TestEngineCanceledContextReturnsPartialResultNotError guards against a real bug: an
// interrupted crawl (e.g. an operator's Ctrl-C canceling the context) used to return
// ctx.Err() alongside the partial Result, and the caller (runner.Run) discarded the whole
// result on any non-nil error — losing everything that had already been crawled. A canceled
// context must now be reported honestly via Coverage.Interrupted instead of as an error.
func TestEngineCanceledContextReturnsPartialResultNotError(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled before the crawl starts

	opts := DefaultOptions()
	engine := New(opts, NewHTTPFetcher(opts))
	result, err := engine.Crawl(ctx, ts.URL)
	if err != nil {
		t.Fatalf("expected no error on a canceled context, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected a non-nil result even when interrupted")
		return
	}
	if !result.Coverage.Interrupted {
		t.Error("expected Coverage.Interrupted to be true")
	}
	if result.Coverage.Complete {
		t.Error("expected Coverage.Complete to be false when interrupted")
	}
}

// TestEngineMaxDurationStopsCrawlEarly guards against the wall-clock budget doing nothing: a
// crawl whose home page links to a slow page must stop once MaxDuration elapses, returning a
// partial result (not an error) with Coverage.DurationLimitReached set.
func TestEngineMaxDurationStopsCrawlEarly(t *testing.T) {
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home</title></head><body><a href="/slow">slow</a></body></html>`)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		<-release // never released during the test; simulates a hang past the budget
		fmt.Fprint(w, `<html><body>too late</body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer func() {
		close(release)
		ts.Close()
	}()

	opts := DefaultOptions()
	opts.MaxDuration = 50 * time.Millisecond
	engine := New(opts, NewHTTPFetcher(opts))

	start := time.Now()
	result, err := engine.Crawl(context.Background(), ts.URL)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error when the duration budget elapses, got: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("crawl took %v, expected it to stop shortly after the 50ms budget", elapsed)
	}
	if !result.Coverage.DurationLimitReached {
		t.Error("expected Coverage.DurationLimitReached to be true")
	}
	if !result.Coverage.Interrupted {
		t.Error("expected Coverage.Interrupted to also be true (DurationLimitReached implies it)")
	}
	if result.Coverage.Complete {
		t.Error("expected Coverage.Complete to be false")
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

func TestThrottleAfter429(t *testing.T) {
	opts := DefaultOptions()
	opts.AdaptiveDelay = true
	engine := New(opts, NewHTTPFetcher(opts))

	page := &Page{StatusCode: http.StatusTooManyRequests, FinalURL: "https://example.test/"}

	// First 429 on an unrestricted crawl drops to the start rate.
	engine.throttleAfter429(page)
	if got := float64(engine.limiter.Limit()); got != backoffStartRate {
		t.Fatalf("after first 429: limit=%v, want %v", got, backoffStartRate)
	}

	// A repeat within the debounce window is ignored.
	engine.throttleAfter429(page)
	if got := float64(engine.limiter.Limit()); got != backoffStartRate {
		t.Fatalf("repeat within debounce changed limit to %v", got)
	}

	// Force the next adjustment past the debounce window; rate halves.
	engine.lastAdjust = engine.lastAdjust.Add(-2 * backoffDebounce)
	engine.throttleAfter429(page)
	if got := float64(engine.limiter.Limit()); got != backoffStartRate/2 {
		t.Fatalf("after second 429: limit=%v, want %v", got, backoffStartRate/2)
	}
}

func TestThrottleAfter429Disabled(t *testing.T) {
	opts := DefaultOptions()
	opts.AdaptiveDelay = false
	opts.RatePerSecond = 5
	engine := New(opts, NewHTTPFetcher(opts))

	engine.throttleAfter429(&Page{StatusCode: http.StatusTooManyRequests})
	if got := float64(engine.limiter.Limit()); got != 5 {
		t.Fatalf("disabled adaptive delay still changed limit to %v", got)
	}
}

func TestThrottleHonorsRetryAfter(t *testing.T) {
	opts := DefaultOptions()
	opts.RatePerSecond = 4 // would otherwise halve to 2 req/s
	engine := New(opts, NewHTTPFetcher(opts))

	page := &Page{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}}
	page.Header.Set("Retry-After", "20") // asks for 0.05 req/s

	engine.throttleAfter429(page)
	if got := float64(engine.limiter.Limit()); got != 0.05 {
		t.Fatalf("Retry-After not honored: limit=%v, want 0.05", got)
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	cases := []struct {
		val  string
		want float64
	}{
		{"", 0},
		{"30", 30},
		{"-5", 0},
		{"garbage", 0},
	}
	for _, c := range cases {
		h := http.Header{}
		if c.val != "" {
			h.Set("Retry-After", c.val)
		}
		if got := retryAfterSeconds(h); got != c.want {
			t.Errorf("retryAfterSeconds(%q)=%v, want %v", c.val, got, c.want)
		}
	}
	if got := retryAfterSeconds(nil); got != 0 {
		t.Errorf("retryAfterSeconds(nil)=%v, want 0", got)
	}
}

// TestRetryAfterSecondsCapsExtremeValues guards against a real denial-of-service surface: an
// uncapped Retry-After let a malicious or misconfigured server (e.g. "Retry-After: 86400")
// stall the entire crawl for as long as it asked, with no overall deadline to recover.
func TestRetryAfterSecondsCapsExtremeValues(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "86400") // 24 hours
	if got, want := retryAfterSeconds(h), maxRetryAfter.Seconds(); got != want {
		t.Errorf("retryAfterSeconds(86400s) = %v, want capped at %v", got, want)
	}

	future := time.Now().Add(24 * time.Hour).UTC().Format(http.TimeFormat)
	h.Set("Retry-After", future)
	if got, want := retryAfterSeconds(h), maxRetryAfter.Seconds(); got > want {
		t.Errorf("retryAfterSeconds(HTTP-date 24h out) = %v, want capped at %v", got, want)
	}
}

// TestThrottleAfter429NeverBelowCappedRetryRate ensures the cap actually reaches the rate
// limiter: throttleAfter429 must not honor an extreme Retry-After by setting a rate slower
// than 1/maxRetryAfter.
func TestThrottleAfter429NeverBelowCappedRetryRate(t *testing.T) {
	opts := DefaultOptions()
	opts.AdaptiveDelay = true
	e := New(opts, NewHTTPFetcher(opts))
	page := &Page{RequestedURL: "https://example.com/", StatusCode: 429, Header: http.Header{
		"Retry-After": []string{"86400"},
	}}
	e.throttleAfter429(page)
	minAllowedRate := 1.0 / maxRetryAfter.Seconds()
	if e.curRate < minAllowedRate {
		t.Errorf("curRate = %v, want >= %v (capped Retry-After)", e.curRate, minAllowedRate)
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
