package webserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return New(store.New(t.TempDir()))
}

func doJSON(t *testing.T, srv *Server, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
	}
	return rec, out
}

func startCrawlTarget(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home</title></head><body>hello</body></html>`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// pollUntilFinished polls GET /api/crawls/{id} until status is no longer "running", failing
// the test if that doesn't happen within the deadline.
func pollUntilFinished(t *testing.T, srv *Server, id string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, job := doJSON(t, srv, http.MethodGet, "/api/crawls/"+id, nil)
		if job["status"] != "running" {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("crawl %s did not finish within the test deadline", id)
	return nil
}

func TestListAnalyzers(t *testing.T) {
	srv := newTestServer(t)
	rec, out := doJSON(t, srv, http.MethodGet, "/api/analyzers", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	analyzers, _ := out["analyzers"].([]any)
	if len(analyzers) == 0 {
		t.Fatal("expected at least one analyzer")
	}
}

func TestStartCrawlPollAndExport(t *testing.T) {
	srv := newTestServer(t)
	target := startCrawlTarget(t)

	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{
		"url":       target.URL,
		"max_pages": 1,
		"save":      true,
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, body = %v", rec.Code, started)
	}
	id, _ := started["id"].(string)
	if id == "" {
		t.Fatalf("missing id in response: %v", started)
	}

	job := pollUntilFinished(t, srv, id)
	if job["status"] != "done" {
		t.Fatalf("status = %v, want done (job=%v)", job["status"], job)
	}
	report, _ := job["report"].(map[string]any)
	if report == nil {
		t.Fatalf("missing report in job: %v", job)
	}
	if report["pages_crawled"].(float64) != 1 {
		t.Errorf("pages_crawled = %v, want 1", report["pages_crawled"])
	}
	if job["persisted"] != true {
		t.Errorf("persisted = %v, want true (save was requested)", job["persisted"])
	}

	// Listing should surface the saved crawl exactly once.
	_, list := doJSON(t, srv, http.MethodGet, "/api/crawls", nil)
	crawls, _ := list["crawls"].([]any)
	if len(crawls) != 1 {
		t.Fatalf("crawls = %v, want exactly 1 entry", crawls)
	}

	for _, format := range []struct {
		name        string
		contentType string
	}{
		{"json", "application/json"},
		{"csv", "text/csv"},
		{"html", "text/html"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/crawls/"+id+"/export?format="+format.name, nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("export %s: status = %d", format.name, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != format.contentType {
			t.Errorf("export %s: content-type = %q, want %q", format.name, ct, format.contentType)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("export %s: empty body", format.name)
		}
	}
}

func TestCancelCrawl(t *testing.T) {
	srv := newTestServer(t)

	block := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	})
	target := httptest.NewServer(mux)
	defer target.Close()
	defer close(block)

	_, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL})
	id := started["id"].(string)

	// Give the crawl a moment to actually be in flight against the blocking handler before
	// canceling, so this exercises mid-crawl cancellation rather than a race at job creation.
	time.Sleep(50 * time.Millisecond)

	req := httptest.NewRequest(http.MethodPost, "/api/crawls/"+id+"/cancel", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("cancel status = %d", rec.Code)
	}

	job := pollUntilFinished(t, srv, id)
	if job["status"] != "canceled" {
		t.Fatalf("status = %v, want canceled (job=%v)", job["status"], job)
	}
}

func TestCancelUnknownCrawlReturnsNotFound(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/crawls/does-not-exist/cancel", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestStartCrawlRejectsEmptyURL(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
