package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

// slashSite is a slash-canonical site, as WordPress is: /x redirects to /x/. Its pages link
// with the trailing slash, as such sites do, plus one link to /a without it. robots.txt
// disallows /private/. It records every path requested.
func slashSite(t *testing.T) (*httptest.Server, func() []string) {
	t.Helper()
	var (
		mu    sync.Mutex
		paths []string
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
			return
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><a href="/a/">a</a><a href="/b/">b</a><a href="/a">a again</a><a href="/private/">p</a></body></html>`))
	}))
	t.Cleanup(ts.Close)
	return ts, func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := append([]string(nil), paths...)
		sort.Strings(out)
		return out
	}
}

// TestEngineFetchesURLsAsWritten: links are fetched with the trailing slash they were written
// with, so a slash-canonical site serves every page without a redirect; /a and /a/ are still
// one page; robots.txt is matched against the path actually requested.
func TestEngineFetchesURLsAsWritten(t *testing.T) {
	ts, requested := slashSite(t)
	opts := DefaultOptions()
	opts.UserAgent = "gocrawl-test"
	r, err := New(opts, NewHTTPFetcher(opts)).Crawl(context.Background(), ts.URL+"/start/")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := strings.Join(requested(), ","), "/a/,/b/,/start/"; got != want {
		t.Errorf("requested %s, want %s (no slash-stripped fetches, /private/ blocked by robots.txt)", got, want)
	}
	for _, p := range r.Pages {
		if len(p.Redirects) > 0 {
			t.Errorf("%s redirected (%v); a URL written with its trailing slash should be fetched as written", p.RequestedURL, p.Redirects)
		}
	}
	if len(r.Pages) != 3 {
		t.Errorf("crawled %d pages, want 3: /a and /a/ are one page", len(r.Pages))
	}
	if _, ok := r.Page(ts.URL + "/a"); !ok {
		t.Error("the page is not found under its slash-less form; lookups must still ignore the trailing slash")
	}
}
