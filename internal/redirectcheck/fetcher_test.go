package redirectcheck_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func TestNewFetcherSendsUserAgentAndRecordsRedirects(t *testing.T) {
	var mu sync.Mutex
	var agents []string
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		agents = append(agents, r.UserAgent())
		mu.Unlock()
		http.Redirect(w, r, "/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		agents = append(agents, r.UserAgent())
		mu.Unlock()
		fmt.Fprint(w, "ok")
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	f := redirectcheck.NewFetcher(5*time.Second, "redirect-test/1.0")
	page, err := f.Fetch(context.Background(), ts.URL+"/old")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if page.StatusCode != http.StatusOK || page.FinalURL != ts.URL+"/new" || len(page.Redirects) != 1 {
		t.Fatalf("page = status %d, final %q, %d redirect(s)", page.StatusCode, page.FinalURL, len(page.Redirects))
	}
	mu.Lock()
	defer mu.Unlock()
	for _, ua := range agents {
		if ua != "redirect-test/1.0" {
			t.Fatalf("User-Agent = %q, want redirect-test/1.0", ua)
		}
	}
	if redirectcheck.DefaultTimeout != 15*time.Second || redirectcheck.DefaultUserAgent == "" {
		t.Fatalf("defaults changed: %v %q", redirectcheck.DefaultTimeout, redirectcheck.DefaultUserAgent)
	}
}
