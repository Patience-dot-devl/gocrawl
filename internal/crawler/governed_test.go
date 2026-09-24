package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func governedTarget(t *testing.T, robots string, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, robots)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		fmt.Fprint(w, "<html><body>x</body></html>")
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, &hits
}

// TestGovernedFetcherHonoursRobots: a fetch that robots.txt disallows must be refused before
// any request reaches the server, exactly as the crawl itself would refuse to enqueue it.
func TestGovernedFetcherHonoursRobots(t *testing.T) {
	ts, hits := governedTarget(t, "User-agent: *\nDisallow: /secret\n", http.StatusOK)
	opts := DefaultOptions()
	opts.RespectRobots = true
	engine := New(opts, NewHTTPFetcher(opts))

	f := engine.Govern(NewHTTPFetcher(opts))
	page, err := f.Fetch(context.Background(), ts.URL+"/secret")
	if !errors.Is(err, ErrRobotsDisallowed) {
		t.Fatalf("err = %v, want ErrRobotsDisallowed", err)
	}
	if page != nil {
		t.Fatalf("page = %+v, want nil", page)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("server hit %d time(s), want 0", n)
	}

	if _, err := f.Fetch(context.Background(), ts.URL+"/public"); err != nil {
		t.Fatalf("allowed path: %v", err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("server hit %d time(s), want 1", n)
	}
}

func TestGovernedFetcherIgnoresRobotsWhenCrawlDoes(t *testing.T) {
	ts, hits := governedTarget(t, "User-agent: *\nDisallow: /\n", http.StatusOK)
	opts := DefaultOptions()
	opts.RespectRobots = false
	engine := New(opts, NewHTTPFetcher(opts))

	if _, err := engine.Govern(NewHTTPFetcher(opts)).Fetch(context.Background(), ts.URL+"/secret"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("server hit %d time(s), want 1", n)
	}
}

// TestGovernedFetcherFeedsAdaptiveLimiter: a 429 seen by an analyzer's fetch must back the
// shared limiter off just like a 429 seen by the crawl, so a probe that trips the server's
// rate limit slows every fetch that follows.
func TestGovernedFetcherFeedsAdaptiveLimiter(t *testing.T) {
	ts, _ := governedTarget(t, "User-agent: *\nAllow: /\n", http.StatusTooManyRequests)
	opts := DefaultOptions()
	opts.AdaptiveDelay = true
	engine := New(opts, NewHTTPFetcher(opts))

	if _, err := engine.Govern(NewHTTPFetcher(opts)).Fetch(context.Background(), ts.URL+"/probe"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if n := engine.limiter.ThrottleCount(); n != 1 {
		t.Fatalf("ThrottleCount = %d, want 1", n)
	}
}
