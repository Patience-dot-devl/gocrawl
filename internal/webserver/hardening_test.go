package webserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

func TestRejectsNonLoopbackHostHeader(t *testing.T) {
	srv := newTestServer(t)
	for _, host := range []string{"evil.example", "evil.example:8080", "192.168.1.5:8080"} {
		req := httptest.NewRequest(http.MethodGet, "/api/analyzers", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("Host %q: status = %d, want 403", host, rec.Code)
		}
	}
	for _, host := range []string{"localhost", "localhost:8080", "127.0.0.1:8080", "[::1]:8080"} {
		req := httptest.NewRequest(http.MethodGet, "/api/analyzers", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Host %q: status = %d, want 200", host, rec.Code)
		}
	}
}

func TestAllowAnyHostDisablesHostCheck(t *testing.T) {
	srv := New(store.New(t.TempDir()), AllowAnyHost())
	req := httptest.NewRequest(http.MethodGet, "/api/analyzers", nil)
	req.Host = "192.168.1.5:8080"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRejectsCrossOriginMutation(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/crawls", strings.NewReader(`{"url":"http://127.0.0.1:1/"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if n := len(srv.jobs.list()); n != 0 {
		t.Fatalf("%d job(s) started despite cross-origin rejection", n)
	}
}

func TestSameOriginMutationAllowed(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/crawls/nope/cancel", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://localhost:8080")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (past the origin check)", rec.Code)
	}
}

func TestStartCrawlRequiresJSONContentType(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/crawls", strings.NewReader(`{"url":"http://127.0.0.1:1/"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestMaxRunningCrawlsReturns429(t *testing.T) {
	block := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	})
	target := httptest.NewServer(mux)
	t.Cleanup(func() { close(block); target.Close() })

	srv := New(store.New(t.TempDir()), WithMaxRunning(1))
	rec, first := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "depth": 0})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("first crawl: status = %d, want 202", rec.Code)
	}
	rec, _ = doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "depth": 0})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second crawl: status = %d, want 429", rec.Code)
	}
	doJSON(t, srv, http.MethodPost, "/api/crawls/"+first["id"].(string)+"/cancel", nil)
	pollUntilFinished(t, srv, first["id"].(string))
	rec, _ = doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "depth": 0})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("after cancel: status = %d, want 202", rec.Code)
	}
}
