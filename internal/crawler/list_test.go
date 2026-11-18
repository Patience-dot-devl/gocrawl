package crawler

import (
	"context"
	"sort"
	"sync/atomic"
	"testing"
)

// TestEngineListModeFetchesOnlyListedURLs: in list mode the crawl fetches the listed URLs and
// nothing they link to, skips a listed URL outside the scope, and still honours MaxPages.
func TestEngineListModeFetchesOnlyListedURLs(t *testing.T) {
	var hits atomic.Int32
	ts := hubSite(t, 5, &hits)

	run := func(maxPages int, urls ...string) *Result {
		t.Helper()
		opts := DefaultOptions()
		opts.UserAgent = "gocrawl-test"
		opts.MaxPages = maxPages
		opts.URLs = urls
		r, err := New(opts, NewHTTPFetcher(opts)).Crawl(context.Background(), ts.URL)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	// The hub page links to /p0../p4; listing it must not pull them in.
	r := run(0, ts.URL+"/p1", ts.URL+"/", ts.URL+"/p3", "https://elsewhere.example/p", ts.URL+"/p1")
	var got []string
	for _, p := range r.Pages {
		got = append(got, p.RequestedURL)
	}
	sort.Strings(got)
	want := []string{ts.URL + "/", ts.URL + "/p1", ts.URL + "/p3"}
	if len(got) != len(want) {
		t.Fatalf("fetched %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fetched %v, want %v", got, want)
		}
	}
	if hits.Load() != 2 {
		t.Errorf("leaf hits = %d, want 2 (only the listed leaves)", hits.Load())
	}
	if !r.Coverage.Complete {
		t.Errorf("coverage incomplete in list mode: %+v", r.Coverage)
	}
	if home, ok := r.Page(ts.URL + "/"); !ok || len(home.Links) != 5 {
		t.Errorf("the hub page's links were not extracted for the analyzers")
	}

	r = run(2, ts.URL+"/p0", ts.URL+"/p1", ts.URL+"/p2")
	if len(r.Pages) != 2 || !r.Coverage.PageLimitReached {
		t.Errorf("MaxPages 2: fetched %d pages (page limit reached: %v), want 2 and true", len(r.Pages), r.Coverage.PageLimitReached)
	}
}
