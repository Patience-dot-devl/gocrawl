package crawler

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// TestEngineOnProgressMatchesRecordedPages: one event per recorded page, delivered in
// recording order (Fetched = 1, 2, 3, ...), so a consumer can append one tape tick per call.
func TestEngineOnProgressMatchesRecordedPages(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	var (
		mu     sync.Mutex
		events []Progress
	)
	opts := DefaultOptions()
	opts.MaxDepth = 1
	opts.Concurrency = 2
	opts.OnProgress = func(p Progress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	}
	engine := New(opts, NewHTTPFetcher(opts))
	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("crawl error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) != len(result.Pages) {
		t.Fatalf("got %d progress events, want one per recorded page (%d)", len(events), len(result.Pages))
	}
	byURL := map[string]Progress{}
	for i, ev := range events {
		if ev.Fetched != i+1 {
			t.Errorf("event %d has Fetched=%d, want %d (events must arrive in recording order)", i, ev.Fetched, i+1)
		}
		byURL[ev.CurrentURL] = ev
	}
	if ev := byURL[ts.URL+"/missing"]; ev.Status != http.StatusNotFound {
		t.Errorf("/missing reported status %d, want 404", ev.Status)
	}
	redir := byURL[ts.URL+"/redir"]
	if redir.Status != http.StatusMovedPermanently {
		t.Errorf("/redir reported status %d, want the first hop's 301", redir.Status)
	}
	if redir.FinalURL != ts.URL+"/target" {
		t.Errorf("/redir FinalURL = %q, want %q", redir.FinalURL, ts.URL+"/target")
	}
	if last := events[len(events)-1]; last.Queued != 0 {
		t.Errorf("last event Queued = %d, want 0", last.Queued)
	}
}

func TestEngineWithoutOnProgressStillCrawls(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	opts := DefaultOptions()
	opts.MaxDepth = 1
	engine := New(opts, NewHTTPFetcher(opts))
	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil || len(result.Pages) == 0 {
		t.Fatalf("crawl without a progress callback: pages=%d err=%v", len(result.Pages), err)
	}
}
