package webserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

// slowSite serves a home page linking to n pages that each take delay to answer.
func slowSite(t *testing.T, n int, delay time.Duration) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			var b strings.Builder
			b.WriteString("<html><head><title>Home</title></head><body>")
			for i := range n {
				fmt.Fprintf(&b, `<a href="/p%d">p%d</a>`, i, i)
			}
			b.WriteString("</body></html>")
			fmt.Fprint(w, b.String())
		case "/robots.txt", "/sitemap.xml", "/sitemap_index.xml", "/llms.txt":
			http.NotFound(w, r)
		default:
			time.Sleep(delay)
			fmt.Fprintf(w, "<html><head><title>%s</title></head><body>page</body></html>", r.URL.Path)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func waitForCrawlState(t *testing.T, srv *Server, id, what string, cond func(job map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, job := doJSON(t, srv, http.MethodGet, "/api/crawls/"+id, nil)
		if cond(job) {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("crawl %s: timed out waiting for %s", id, what)
	return nil
}

func fetchedOf(job map[string]any) float64 {
	p, _ := job["progress"].(map[string]any)
	f, _ := p["fetched"].(float64)
	return f
}

// serveCrawlRequest sends one request without a *testing.T, for use from helper goroutines.
func serveCrawlRequest(srv *Server, method, path, contentType, body string) int {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "localhost:8080"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec.Code
}

func startSlowCrawl(t *testing.T, srv *Server, target *httptest.Server) string {
	t.Helper()
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "concurrency": 1, "max_pages": 0})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start status %d, body %v", rec.Code, started)
	}
	id, _ := started["id"].(string)
	waitForCrawlState(t, srv, id, "the first pages", func(j map[string]any) bool { return fetchedOf(j) >= 2 })
	return id
}

func pauseAndWait(t *testing.T, srv *Server, id string) map[string]any {
	t.Helper()
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/pause", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("pause status %d, want 204", rec.Code)
	}
	return waitForCrawlState(t, srv, id, "paused status", func(j map[string]any) bool { return j["status"] == "paused" })
}

func TestPauseAndResumeCrawl(t *testing.T) {
	srv := newTestServer(t)
	id := startSlowCrawl(t, srv, slowSite(t, 30, 30*time.Millisecond))

	job := pauseAndWait(t, srv, id)
	prog := job["progress"].(map[string]any)
	if prog["paused"] != true || prog["pause_reason"] != "user" {
		t.Errorf("progress after pause = %v", prog)
	}
	time.Sleep(150 * time.Millisecond) // an in-flight page may still land
	_, j1 := doJSON(t, srv, http.MethodGet, "/api/crawls/"+id, nil)
	time.Sleep(300 * time.Millisecond)
	_, j2 := doJSON(t, srv, http.MethodGet, "/api/crawls/"+id, nil)
	if fetchedOf(j2) != fetchedOf(j1) {
		t.Fatalf("fetched rose from %v to %v while paused", fetchedOf(j1), fetchedOf(j2))
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/pause", nil); rec.Code != http.StatusConflict {
		t.Errorf("second pause: status %d, want 409", rec.Code)
	}

	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/resume", map[string]any{"rate": 50}); rec.Code != http.StatusNoContent {
		t.Fatalf("resume status %d, want 204", rec.Code)
	}
	resumed := waitForCrawlState(t, srv, id, "running status", func(j map[string]any) bool { return j["status"] != "paused" })
	if p := resumed["progress"].(map[string]any); p["paused"] != false || p["rate"] != float64(50) {
		t.Errorf("progress after resume = %v, want unpaused at rate 50", p)
	}
	// Running or already done by now: either way it is not paused.
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/resume", map[string]any{}); rec.Code != http.StatusConflict {
		t.Errorf("resume of a crawl that isn't paused: status %d, want 409", rec.Code)
	}

	done := pollUntilFinished(t, srv, id)
	if done["status"] != "done" {
		t.Fatalf("final status %v, want done", done["status"])
	}
	if rep := done["report"].(map[string]any); rep["pages_crawled"] != float64(31) {
		t.Errorf("pages_crawled = %v, want 31", rep["pages_crawled"])
	}
}

func TestPauseResumeRejectWrongStates(t *testing.T) {
	srv := newTestServer(t)
	target := startCrawlTarget(t)
	_, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "max_pages": 1})
	id := started["id"].(string)
	pollUntilFinished(t, srv, id)

	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/pause", nil); rec.Code != http.StatusConflict {
		t.Errorf("pause finished crawl: %d, want 409", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/resume", map[string]any{}); rec.Code != http.StatusConflict {
		t.Errorf("resume finished crawl: %d, want 409", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/nope/pause", nil); rec.Code != http.StatusNotFound {
		t.Errorf("pause unknown crawl: %d, want 404", rec.Code)
	}
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/nope/resume", map[string]any{}); rec.Code != http.StatusNotFound {
		t.Errorf("resume unknown crawl: %d, want 404", rec.Code)
	}
	// Error messages never echo the crawl id from the URL.
	for _, c := range []struct {
		path string
		body any
	}{
		{"/api/crawls/nope/pause", nil},
		{"/api/crawls/nope/resume", map[string]any{}},
		{"/api/crawls/" + id + "/pause", nil},
		{"/api/crawls/" + id + "/resume", map[string]any{}},
	} {
		_, body := doJSON(t, srv, http.MethodPost, c.path, c.body)
		msg, _ := body["error"].(string)
		if msg == "" || strings.Contains(msg, "nope") || strings.Contains(msg, id) {
			t.Errorf("%s: error %q should be non-empty and not echo the crawl id", c.path, msg)
		}
	}
}

func TestResumeValidatesBody(t *testing.T) {
	srv := newTestServer(t)
	id := startSlowCrawl(t, srv, slowSite(t, 30, 30*time.Millisecond))
	pauseAndWait(t, srv, id)

	for _, rate := range []float64{-1, 0, 0.0005, 1000.5, 1e9} {
		rec, body := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/resume", map[string]any{"rate": rate})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("rate %v: %d, want 400", rate, rec.Code)
		}
		if msg, _ := body["error"].(string); !strings.Contains(msg, "0.001") || !strings.Contains(msg, "1000") {
			t.Errorf("rate %v: error %q should state the allowed range", rate, msg)
		}
	}
	if code := serveCrawlRequest(srv, http.MethodPost, "/api/crawls/"+id+"/resume", "text/plain", `{}`); code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: %d, want 415", code)
	}
	if code := serveCrawlRequest(srv, http.MethodPost, "/api/crawls/"+id+"/resume", "application/json", `{`); code != http.StatusBadRequest {
		t.Errorf("malformed JSON: %d, want 400", code)
	}
	doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/cancel", nil)
	pollUntilFinished(t, srv, id)
}

func TestCancelPausedCrawl(t *testing.T) {
	srv := newTestServer(t)
	id := startSlowCrawl(t, srv, slowSite(t, 30, 30*time.Millisecond))
	pauseAndWait(t, srv, id)
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/cancel", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("cancel paused crawl: %d, want 204", rec.Code)
	}
	done := pollUntilFinished(t, srv, id)
	if done["status"] != "canceled" {
		t.Fatalf("status %v, want canceled", done["status"])
	}
	cov := done["report"].(map[string]any)["coverage"].(map[string]any)
	if cov["interrupted"] != true {
		t.Errorf("coverage = %v, want interrupted", cov)
	}
}

func TestPausedCrawlCountsTowardTheCap(t *testing.T) {
	srv := New(store.New(t.TempDir()), WithMaxRunning(1))
	target := slowSite(t, 30, 30*time.Millisecond)
	id := startSlowCrawl(t, srv, target)
	pauseAndWait(t, srv, id)
	if rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("start while one crawl is paused at cap 1: %d, want 429", rec.Code)
	}
	doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/cancel", nil)
	pollUntilFinished(t, srv, id)
}

func TestPauseIsGuarded(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/crawls/x/pause", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin pause: %d, want 403", rec.Code)
	}
}

func TestProgressMovesJobBetweenRunningAndPaused(t *testing.T) {
	srv := newTestServer(t)
	job, _ := srv.crawls.start(crawlJob{Seed: "https://example.com/"}, func() {}, srv.maxRunning)
	status := func() Status {
		j, _ := srv.crawls.get(job.ID)
		return j.Status
	}

	srv.recordProgress(job.ID, runner.Progress{
		Progress: crawler.Progress{Fetched: 1, Status: 429, Paused: true, PauseReason: crawler.PauseReasonRateLimited, RetryAfterSeconds: 120},
		Phase:    runner.PhaseCrawling,
	})
	if got := status(); got != StatusPaused {
		t.Fatalf("after an auto-pause event: %s, want paused", got)
	}
	srv.recordProgress(job.ID, runner.Progress{Progress: crawler.Progress{Fetched: 1}, Phase: runner.PhaseCrawling})
	if got := status(); got != StatusRunning {
		t.Fatalf("after a resume event: %s, want running", got)
	}

	// A pause queued during the sitemap phase is not undone by a sitemap-phase event.
	srv.crawls.with(job.ID, func(j *Job[crawlJob]) { j.Status = StatusPaused })
	srv.recordProgress(job.ID, runner.Progress{Phase: runner.PhaseSitemap})
	if got := status(); got != StatusPaused {
		t.Fatalf("after a sitemap-phase event: %s, want still paused", got)
	}

	srv.crawls.finish(job.ID, nil, nil)
	srv.recordProgress(job.ID, runner.Progress{Progress: crawler.Progress{Fetched: 1, Paused: true}, Phase: runner.PhaseCrawling})
	if got := status(); got != StatusDone {
		t.Fatalf("a finished job moved to %s", got)
	}
}

// TestPauseResumeWhilePollingDoesNotDeadlock exercises the lock order: pause/resume emit
// progress events (which take the job manager's lock) while GETs hold that lock to build views
// that call Control.Active.
func TestPauseResumeWhilePollingDoesNotDeadlock(t *testing.T) {
	srv := newTestServer(t)
	id := startSlowCrawl(t, srv, slowSite(t, 60, 10*time.Millisecond))

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					serveCrawlRequest(srv, http.MethodGet, "/api/crawls/"+id, "", "")
				}
			}
		}()
		for range 10 {
			serveCrawlRequest(srv, http.MethodPost, "/api/crawls/"+id+"/pause", "", "")
			serveCrawlRequest(srv, http.MethodPost, "/api/crawls/"+id+"/resume", "application/json", `{}`)
		}
		close(stop)
		wg.Wait()
	}()
	select {
	case <-finished:
	case <-time.After(15 * time.Second):
		t.Fatal("pause/resume under concurrent polling deadlocked")
	}
	doJSON(t, srv, http.MethodPost, "/api/crawls/"+id+"/cancel", nil)
	pollUntilFinished(t, srv, id)
}
