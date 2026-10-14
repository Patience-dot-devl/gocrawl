package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// hubSite serves a home page linking to n leaf pages and counts leaf fetches.
func hubSite(t *testing.T, n int, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			var b strings.Builder
			b.WriteString("<html><head><title>Hub</title></head><body>")
			for i := range n {
				fmt.Fprintf(&b, `<a href="/p%d">p%d</a>`, i, i)
			}
			b.WriteString("</body></html>")
			fmt.Fprint(w, b.String())
		case "/robots.txt":
			http.NotFound(w, r)
		default:
			hits.Add(1)
			fmt.Fprint(w, "<html><head><title>Leaf</title></head><body>leaf</body></html>")
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// startPausable starts a Concurrency-1 crawl and returns once the hub page is recorded.
func startPausable(t *testing.T, ts *httptest.Server, ctx context.Context, onProgress func(Progress)) (*Engine, chan *Result) {
	t.Helper()
	first := make(chan struct{}, 1)
	opts := DefaultOptions()
	opts.Concurrency = 1
	opts.OnProgress = func(p Progress) {
		if onProgress != nil {
			onProgress(p)
		}
		if p.Fetched == 1 {
			select {
			case first <- struct{}{}:
			default:
			}
		}
	}
	e := New(opts, NewHTTPFetcher(opts))
	done := make(chan *Result, 1)
	go func() {
		r, _ := e.Crawl(ctx, ts.URL)
		done <- r
	}()
	select {
	case <-first:
	case <-time.After(5 * time.Second):
		t.Fatal("the hub page was never recorded")
	}
	return e, done
}

func TestEnginePauseStopsNewFetchesAndResumeFinishes(t *testing.T) {
	var hits atomic.Int32
	ts := hubSite(t, 20, &hits)
	var (
		mu     sync.Mutex
		events []Progress
	)
	e, done := startPausable(t, ts, context.Background(), func(p Progress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	})

	e.Pause()
	if !e.Paused() {
		t.Fatal("Paused() = false right after Pause()")
	}
	time.Sleep(100 * time.Millisecond) // a fetch already past the frontier may still land
	before := hits.Load()
	time.Sleep(300 * time.Millisecond)
	if after := hits.Load(); after != before {
		t.Fatalf("%d leaf pages fetched while paused", after-before)
	}
	if before > 1 {
		t.Errorf("%d leaf pages fetched after Pause, want at most the one already in flight", before)
	}
	select {
	case <-done:
		t.Fatal("the crawl returned while paused")
	default:
	}

	e.Resume(0)
	var r *Result
	select {
	case r = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the crawl did not finish after Resume")
	}
	if len(r.Pages) != 21 || hits.Load() != 20 || !r.Coverage.Complete {
		t.Errorf("pages=%d hits=%d complete=%v, want 21 / 20 / true", len(r.Pages), hits.Load(), r.Coverage.Complete)
	}

	mu.Lock()
	defer mu.Unlock()
	sawPause, sawResume := false, false
	for _, ev := range events {
		if ev.Paused && ev.PauseReason == PauseReasonUser {
			sawPause = true
		}
		if sawPause && !ev.Paused {
			sawResume = true
		}
	}
	if !sawPause || !sawResume {
		t.Errorf("progress events: saw pause %v, saw resume %v; want both", sawPause, sawResume)
	}
}

func TestEngineCancelWhilePausedReturnsPartialResult(t *testing.T) {
	var hits atomic.Int32
	ts := hubSite(t, 20, &hits)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e, done := startPausable(t, ts, ctx, nil)
	e.Pause()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case r := <-done:
		if r == nil || !r.Coverage.Interrupted || len(r.Pages) < 1 || len(r.Pages) > 2 {
			t.Fatalf("result after cancel while paused: %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not wake the paused crawl")
	}
}

func TestEngineMaxDurationExcludesPausedTime(t *testing.T) {
	var hits atomic.Int32
	ts := hubSite(t, 1, &hits)
	opts := DefaultOptions()
	opts.MaxDuration = 250 * time.Millisecond
	e := New(opts, NewHTTPFetcher(opts))
	e.Pause() // queued: the crawl starts paused
	done := make(chan *Result, 1)
	go func() {
		r, _ := e.Crawl(context.Background(), ts.URL)
		done <- r
	}()
	time.Sleep(600 * time.Millisecond) // well past MaxDuration, all of it paused
	e.Resume(0)
	select {
	case r := <-done:
		if r.Coverage.DurationLimitReached || !r.Coverage.Complete || len(r.Pages) != 2 {
			t.Fatalf("coverage=%+v pages=%d, want a complete 2-page crawl", r.Coverage, len(r.Pages))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the crawl did not finish after Resume")
	}
	if at := e.ActiveTime(); at >= 250*time.Millisecond {
		t.Errorf("ActiveTime = %v, want the 600ms pause excluded", at)
	}
}

func TestEnginePauseAfterCrawlIsNoOp(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	opts := DefaultOptions()
	opts.MaxDepth = 1
	e := New(opts, NewHTTPFetcher(opts))
	if _, err := e.Crawl(context.Background(), ts.URL); err != nil {
		t.Fatalf("crawl: %v", err)
	}
	e.Pause()
	if e.Paused() {
		t.Error("Pause after the crawl ended should do nothing")
	}
	e.Resume(5) // must not block or panic
}

func TestEngineResumeAppliesNewRate(t *testing.T) {
	opts := DefaultOptions()
	e := New(opts, NewHTTPFetcher(opts))
	e.Pause()
	e.Resume(2.5)
	if got := e.limiter.EffectiveRate(); got != 2.5 {
		t.Errorf("EffectiveRate after Resume(2.5) = %v, want 2.5", got)
	}
	e.Pause()
	e.Resume(0)
	if got := e.limiter.EffectiveRate(); got != 2.5 {
		t.Errorf("Resume(0) changed the rate to %v, want it kept at 2.5", got)
	}
}

func TestEnginePauseHandsBackATaskWaitingOnTheRateLimiter(t *testing.T) {
	var (
		mu    sync.Mutex
		paths []string
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><body><a href="/p0">p0</a><a href="/p1">p1</a></body></html>`)
		case "/robots.txt":
			http.NotFound(w, r)
		default:
			mu.Lock()
			paths = append(paths, r.URL.Path)
			mu.Unlock()
			fmt.Fprint(w, "<html><body>leaf</body></html>")
		}
	}))
	defer ts.Close()

	first := make(chan struct{}, 1)
	opts := DefaultOptions()
	opts.Concurrency = 1
	opts.RatePerSecond = 2 // the worker waits ~500ms for its token after the hub page
	opts.OnProgress = func(p Progress) {
		if p.Fetched == 1 {
			select {
			case first <- struct{}{}:
			default:
			}
		}
	}
	e := New(opts, NewHTTPFetcher(opts))
	done := make(chan *Result, 1)
	go func() {
		r, _ := e.Crawl(context.Background(), ts.URL)
		done <- r
	}()
	select {
	case <-first:
	case <-time.After(5 * time.Second):
		t.Fatal("the hub page was never recorded")
	}
	e.Pause()
	time.Sleep(900 * time.Millisecond) // past the token wait
	mu.Lock()
	fetchedWhilePaused := len(paths)
	mu.Unlock()
	if fetchedWhilePaused != 0 {
		t.Fatalf("%d leaf pages fetched while paused, want the waiting task handed back", fetchedWhilePaused)
	}

	e.Resume(0)
	select {
	case r := <-done:
		if len(r.Pages) != 3 || !r.Coverage.Complete {
			t.Fatalf("pages=%d complete=%v, want a complete 3-page crawl", len(r.Pages), r.Coverage.Complete)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the crawl did not finish after Resume")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 2 || paths[0] != "/p0" {
		t.Errorf("leaf fetch order = %v, want /p0 (the handed-back task) first", paths)
	}
}

func TestEngineCrawlEndingWhilePausedReportsUnpaused(t *testing.T) {
	var hits atomic.Int32
	ts := hubSite(t, 3, &hits)
	var (
		mu   sync.Mutex
		last Progress
		n    int
	)
	opts := DefaultOptions()
	opts.OnProgress = func(p Progress) {
		mu.Lock()
		last, n = p, n+1
		mu.Unlock()
	}
	e := New(opts, NewHTTPFetcher(opts))
	e.Pause() // queued: the crawl starts paused and never fetches
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan *Result, 1)
	go func() {
		r, _ := e.Crawl(ctx, ts.URL)
		done <- r
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case r := <-done:
		if len(r.Pages) != 0 || !r.Coverage.Interrupted {
			t.Fatalf("pages=%d interrupted=%v, want 0 pages and an interrupted crawl", len(r.Pages), r.Coverage.Interrupted)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not end a crawl that started paused")
	}
	if e.Paused() {
		t.Error("Paused() = true after Crawl returned")
	}
	mu.Lock()
	defer mu.Unlock()
	if n < 2 || last.Paused {
		t.Errorf("got %d events, last paused=%v; want the pause event and a final unpaused one", n, last.Paused)
	}
	if hits.Load() != 0 {
		t.Errorf("%d leaf pages fetched by a crawl that never resumed", hits.Load())
	}
}

// TestEnginePauseResumeConcurrent toggles pause from several goroutines while a crawl runs,
// for the race detector and to catch lock-order deadlocks between Pause/Resume and progress.
func TestEnginePauseResumeConcurrent(t *testing.T) {
	var hits atomic.Int32
	ts := hubSite(t, 40, &hits)
	opts := DefaultOptions()
	opts.Concurrency = 4
	opts.MaxDuration = time.Minute
	var events atomic.Int32
	opts.OnProgress = func(Progress) { events.Add(1) }
	e := New(opts, NewHTTPFetcher(opts))
	done := make(chan *Result, 1)
	go func() {
		r, _ := e.Crawl(context.Background(), ts.URL)
		done <- r
	}()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				if i%2 == 0 {
					e.Pause()
				} else {
					e.Resume(float64(i-1) * 500) // 0 (keep) or 1000 req/s
				}
				_ = e.Paused()
				_ = e.ActiveTime()
				time.Sleep(time.Millisecond)
			}
		})
	}
	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
	e.Resume(0)
	select {
	case r := <-done:
		if len(r.Pages) != 41 || !r.Coverage.Complete || r.Coverage.DurationLimitReached {
			t.Fatalf("pages=%d coverage=%+v, want a complete 41-page crawl", len(r.Pages), r.Coverage)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the crawl did not finish after the final Resume")
	}
	if events.Load() < 41 {
		t.Errorf("%d progress events, want at least one per page", events.Load())
	}
}

func TestMaybeAutoPauseNeedsOptionBackoffsAndThreshold(t *testing.T) {
	burst := func(e *Engine, n int) {
		for range n {
			e.limiter.OnResponse(http.StatusTooManyRequests, nil)
		}
	}

	off := DefaultOptions()
	e := New(off, NewHTTPFetcher(off))
	e.limiter.debounce = 0
	burst(e, 10)
	e.maybeAutoPause(http.StatusTooManyRequests)
	if e.Paused() {
		t.Error("paused without PauseOnThrottle")
	}

	on := DefaultOptions()
	on.PauseOnThrottle = true

	// With adaptive delay off there are no backoffs to wait for: ten throttle responses in the
	// window are enough.
	noBackoff := on
	noBackoff.AdaptiveDelay = false
	e = New(noBackoff, NewHTTPFetcher(noBackoff))
	burst(e, 9)
	e.maybeAutoPause(http.StatusTooManyRequests)
	if e.Paused() {
		t.Error("paused below the threshold of 10 with adaptive delay off")
	}
	burst(e, 1)
	e.maybeAutoPause(http.StatusTooManyRequests)
	if !e.Paused() || e.ps.reason != PauseReasonRateLimited {
		t.Errorf("adaptive delay off: paused=%v reason=%q, want paused for %q", e.Paused(), e.ps.reason, PauseReasonRateLimited)
	}

	e = New(on, NewHTTPFetcher(on)) // default 2s debounce: ten rapid 429s are one backoff
	burst(e, 10)
	e.maybeAutoPause(http.StatusTooManyRequests)
	if e.Paused() {
		t.Error("paused after a single backoff")
	}

	e = New(on, NewHTTPFetcher(on))
	e.limiter.debounce = 0
	burst(e, 9)
	e.maybeAutoPause(http.StatusTooManyRequests)
	if e.Paused() {
		t.Error("paused below the threshold of 10")
	}
	burst(e, 1)
	e.maybeAutoPause(http.StatusOK)
	if e.Paused() {
		t.Error("a 200 must not trigger auto-pause")
	}
	e.maybeAutoPause(http.StatusTooManyRequests)
	if !e.Paused() || e.ps.reason != PauseReasonRateLimited {
		t.Errorf("paused=%v reason=%q, want paused for %q", e.Paused(), e.ps.reason, PauseReasonRateLimited)
	}
}

// TestEngineAutoPausesOnSustained429 runs a real crawl against a server that answers 429 to
// everything but the home page, and expects the crawl to pause itself, report the server's
// Retry-After, stop fetching, and still end cleanly when canceled.
func TestEngineAutoPausesOnSustained429(t *testing.T) {
	var leafHits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			var b strings.Builder
			b.WriteString("<html><body>")
			for i := range 40 {
				fmt.Fprintf(&b, `<a href="/p%d">p%d</a>`, i, i)
			}
			b.WriteString("</body></html>")
			fmt.Fprint(w, b.String())
		case "/robots.txt":
			http.NotFound(w, r)
		default:
			// Only the tenth refusal names a long wait: earlier ones would otherwise slow the
			// test to one request every two minutes.
			if leafHits.Add(1) >= 10 {
				w.Header().Set("Retry-After", "120")
			}
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer ts.Close()

	opts := DefaultOptions()
	opts.Concurrency = 1
	opts.RatePerSecond = 4000 // nine halvings from here still leave ~8 req/s
	opts.PauseOnThrottle = true
	paused := make(chan Progress, 1)
	opts.OnProgress = func(p Progress) {
		if p.Paused {
			select {
			case paused <- p:
			default:
			}
		}
	}
	e := New(opts, NewHTTPFetcher(opts))
	e.limiter.debounce = 0 // back off on every 429, not once per 2s

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan *Result, 1)
	go func() {
		r, _ := e.Crawl(ctx, ts.URL)
		done <- r
	}()

	var p Progress
	select {
	case p = <-paused:
	case <-time.After(10 * time.Second):
		t.Fatal("the crawl never paused itself")
	}
	if p.PauseReason != PauseReasonRateLimited || p.RetryAfterSeconds != 120 || p.Throttled < 10 {
		t.Errorf("pause event = %+v, want reason rate_limited, Retry-After 120, >= 10 throttled", p)
	}
	atPause := leafHits.Load()
	time.Sleep(300 * time.Millisecond)
	if got := leafHits.Load(); got != atPause {
		t.Errorf("%d more requests after the auto-pause", got-atPause)
	}

	cancel()
	select {
	case r := <-done:
		if !r.Coverage.Interrupted || len(r.Pages) != int(atPause)+1 {
			t.Errorf("interrupted=%v pages=%d, want true / %d", r.Coverage.Interrupted, len(r.Pages), atPause+1)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not end the auto-paused crawl")
	}
}

func TestResumeClearsTheStaleRetryAfter(t *testing.T) {
	opts := DefaultOptions()
	e := New(opts, NewHTTPFetcher(opts))
	e.limiter.OnResponse(http.StatusTooManyRequests, http.Header{"Retry-After": {"120"}})
	if got := e.limiter.LastRetryAfter(); got != 120 {
		t.Fatalf("LastRetryAfter = %d, want 120", got)
	}
	e.Pause()
	e.Resume(0)
	if got := e.limiter.LastRetryAfter(); got != 0 {
		t.Errorf("LastRetryAfter after Resume = %d, want 0 so a second auto-pause can't report it", got)
	}
}
