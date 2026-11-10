package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/temoto/robotstxt"
)

func TestHostPacerReservesSlotsPerHost(t *testing.T) {
	now := time.Unix(1000, 0)
	p := newHostPacer()
	p.now = func() time.Time { return now }

	if d := p.reserve("a.example", 0); d != 0 {
		t.Errorf("zero delay waited %v, want 0", d)
	}
	for i, want := range []time.Duration{0, 2 * time.Second, 4 * time.Second} {
		if d := p.reserve("a.example", 2*time.Second); d != want {
			t.Errorf("reservation %d on a.example waited %v, want %v", i, d, want)
		}
	}
	if d := p.reserve("b.example", 2*time.Second); d != 0 {
		t.Errorf("another host waited %v, want 0: hosts must not share a pace", d)
	}
	// Once the booked slots are in the past, the next request may start at once.
	now = now.Add(10 * time.Second)
	if d := p.reserve("a.example", 2*time.Second); d != 0 {
		t.Errorf("after the backlog cleared, waited %v, want 0", d)
	}
}

func TestHostPacerCapsDelay(t *testing.T) {
	now := time.Unix(1000, 0)
	p := newHostPacer()
	p.now = func() time.Time { return now }
	p.reserve("a.example", time.Hour)
	if d := p.reserve("a.example", time.Hour); d != MaxCrawlDelay {
		t.Errorf("second request waited %v, want the %v cap", d, MaxCrawlDelay)
	}
}

func TestHostPacerWaitStopsOnCancel(t *testing.T) {
	p := newHostPacer()
	p.reserve("a.example", MaxCrawlDelay)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.wait(ctx, "a.example", MaxCrawlDelay); err == nil {
		t.Error("wait on a canceled context returned nil")
	}
}

func TestHostPacerBusyWhileWaiting(t *testing.T) {
	p := newHostPacer()
	p.reserve("a.example", 50*time.Millisecond) // the next caller must wait ~50ms
	done := make(chan struct{})
	go func() {
		defer close(done)
		if slept, err := p.wait(context.Background(), "a.example", 50*time.Millisecond); err != nil || !slept {
			t.Errorf("wait = %v, %v; want slept, nil", slept, err)
		}
	}()
	deadline := time.Now().Add(time.Second)
	for !p.busy("a.example") {
		if time.Now().After(deadline) {
			t.Fatal("host never reported busy while a caller waited")
		}
		time.Sleep(time.Millisecond)
	}
	if p.busy("b.example") {
		t.Error("an idle host reported busy")
	}
	<-done
	if p.busy("a.example") {
		t.Error("host still busy after the wait ended")
	}
}

// crawlDelayServer serves a robots.txt with the given body and n linked pages, recording when
// each page request arrived.
func crawlDelayServer(t *testing.T, robots string, n int) (*httptest.Server, func() []time.Time) {
	t.Helper()
	var (
		mu    sync.Mutex
		times []time.Time
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, robots)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		times = append(times, time.Now())
		mu.Unlock()
		if r.URL.Path != "/" {
			fmt.Fprint(w, `<html><body>leaf</body></html>`)
			return
		}
		fmt.Fprint(w, `<html><body>`)
		for i := range n {
			fmt.Fprintf(w, `<a href="/p%d">p</a>`, i)
		}
		fmt.Fprint(w, `</body></html>`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, func() []time.Time {
		mu.Lock()
		defer mu.Unlock()
		out := append([]time.Time(nil), times...)
		sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
		return out
	}
}

func TestEngineObeysCrawlDelay(t *testing.T) {
	const delay = 150 * time.Millisecond
	ts, times := crawlDelayServer(t, "User-agent: *\nCrawl-delay: 0.15\n", 3)

	opts := DefaultOptions()
	opts.Concurrency = 4
	opts.UserAgent = "gocrawl-test"
	if _, err := New(opts, NewHTTPFetcher(opts)).Crawl(context.Background(), ts.URL); err != nil {
		t.Fatal(err)
	}
	got := times()
	if len(got) != 4 {
		t.Fatalf("fetched %d pages, want 4", len(got))
	}
	for i := 1; i < len(got); i++ {
		// Allow a little timer slack below the requested spacing.
		if gap := got[i].Sub(got[i-1]); gap < delay-10*time.Millisecond {
			t.Errorf("requests %d and %d were %v apart, want at least %v", i-1, i, gap, delay)
		}
	}
}

func TestEngineIgnoresCrawlDelayWithoutRespectRobots(t *testing.T) {
	ts, times := crawlDelayServer(t, "User-agent: *\nCrawl-delay: 30\n", 3)

	opts := DefaultOptions()
	opts.Concurrency = 4
	opts.RespectRobots = false
	start := time.Now()
	if _, err := New(opts, NewHTTPFetcher(opts)).Crawl(context.Background(), ts.URL); err != nil {
		t.Fatal(err)
	}
	if n := len(times()); n != 4 {
		t.Fatalf("fetched %d pages, want 4", n)
	}
	if el := time.Since(start); el > 5*time.Second {
		t.Errorf("crawl took %v; Crawl-delay must not apply when robots.txt is not respected", el)
	}
}

func TestGovernedFetcherObeysCrawlDelay(t *testing.T) {
	const delay = 150 * time.Millisecond
	ts, times := crawlDelayServer(t, "User-agent: *\nCrawl-delay: 0.15\n", 0)

	opts := DefaultOptions()
	e := New(opts, NewHTTPFetcher(opts))
	if _, err := e.SetSeed(ts.URL); err != nil {
		t.Fatal(err)
	}
	g := e.Govern(NewHTTPFetcher(opts))
	for i := range 3 {
		if _, err := g.Fetch(context.Background(), fmt.Sprintf("%s/extra%d", ts.URL, i)); err != nil {
			t.Fatal(err)
		}
	}
	got := times()
	for i := 1; i < len(got); i++ {
		if gap := got[i].Sub(got[i-1]); gap < delay-10*time.Millisecond {
			t.Errorf("governed requests %d and %d were %v apart, want at least %v", i-1, i, gap, delay)
		}
	}
}

func TestRobotsDataCrawlDelay(t *testing.T) {
	var nilData *RobotsData
	if d := nilData.CrawlDelay("x"); d != 0 {
		t.Errorf("nil robots: %v", d)
	}
	if d := (&RobotsData{Status: 404}).CrawlDelay("x"); d != 0 {
		t.Errorf("robots not found: %v", d)
	}
	parsed, err := robotstxt.FromString("User-agent: gocrawl\nCrawl-delay: 2\n\nUser-agent: *\nCrawl-delay: 9\n")
	if err != nil {
		t.Fatal(err)
	}
	r := &RobotsData{Found: true, Status: 200, data: parsed}
	if d := r.CrawlDelay("gocrawl/0.1"); d != 2*time.Second {
		t.Errorf("gocrawl group: %v, want 2s", d)
	}
	if d := r.CrawlDelay("otherbot"); d != 9*time.Second {
		t.Errorf("wildcard group: %v, want 9s", d)
	}
}

// TestEngineSlowHostDoesNotBlockOthers is the head-of-line regression: a host with a long
// Crawl-delay must not hold every worker while another host's pages wait in the queue.
func TestEngineSlowHostDoesNotBlockOthers(t *testing.T) {
	var (
		mu       sync.Mutex
		fastDone time.Time
		fastSeen int
	)
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		fastSeen++
		fastDone = time.Now()
		mu.Unlock()
		fmt.Fprint(w, `<html><body>fast</body></html>`)
	}))
	t.Cleanup(fast.Close)

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nCrawl-delay: 0.2\n")
			return
		}
		if r.URL.Path != "/" {
			fmt.Fprint(w, `<html><body>leaf</body></html>`)
			return
		}
		fmt.Fprint(w, `<html><body>`)
		for i := range 8 {
			fmt.Fprintf(w, `<a href="/s%d">s</a>`, i)
		}
		for i := range 4 {
			fmt.Fprintf(w, `<a href="%s/f%d">f</a>`, fast.URL, i)
		}
		fmt.Fprint(w, `</body></html>`)
	}))
	t.Cleanup(slow.Close)

	opts := DefaultOptions()
	opts.Concurrency = 4
	opts.FollowExternal = true
	opts.UserAgent = "gocrawl-test"
	start := time.Now()
	if _, err := New(opts, NewHTTPFetcher(opts)).Crawl(context.Background(), slow.URL); err != nil {
		t.Fatal(err)
	}
	total := time.Since(start)
	mu.Lock()
	defer mu.Unlock()
	if fastSeen != 4 {
		t.Fatalf("fast host served %d pages, want 4", fastSeen)
	}
	// The slow host needs about 8 x 0.2s after the seed. Its links come first in the queue,
	// but the fast host's pages must not wait behind them: before the fix, every worker sat on
	// a slow-host slot and the fast pages started only after about 1s.
	if el := fastDone.Sub(start); el > 400*time.Millisecond {
		t.Errorf("fast host finished %v after start (crawl took %v); it waited on the slow host", el, total)
	}
}
