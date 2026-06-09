package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home Page Title</title></head><body>
			<a href="/page2">Page 2</a>
			<a href="/missing">Missing</a>
			<a href="/redir">Redirect</a>
			<a href="https://external.example/elsewhere">External</a>
		</body></html>`)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Second Page Here</title></head><body>ok</body></html>`)
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/redir", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/target", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/target", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Redirect Target</title></head><body>arrived</body></html>`)
	})
	return httptest.NewServer(mux)
}

func TestEngineCrawl(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	opts := DefaultOptions()
	opts.MaxDepth = 1
	opts.Concurrency = 2
	opts.UserAgent = "gocrawl-test"
	engine := New(opts, NewHTTPFetcher(opts))

	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("crawl error: %v", err)
	}

	// External host should not be crawled.
	for _, p := range result.Pages {
		if got := p.FinalURL; got != "" && hostOf(got) == "external.example" {
			t.Errorf("external host was crawled: %s", got)
		}
	}

	home, ok := result.Page(ts.URL)
	if !ok {
		t.Fatal("home page not found in result")
	}
	if home.StatusCode != 200 || len(home.Links) < 3 {
		t.Errorf("home: status=%d links=%d", home.StatusCode, len(home.Links))
	}

	missing, ok := result.Page(ts.URL + "/missing")
	if !ok || missing.StatusCode != 404 {
		t.Errorf("missing page: ok=%v status=%d", ok, missing.StatusCode)
	}

	redir, ok := result.Page(ts.URL + "/redir")
	if !ok {
		t.Fatal("redirect page not found")
	}
	if len(redir.Redirects) == 0 {
		t.Error("expected redirect chain to be captured")
	}
	if redir.StatusCode != 200 {
		t.Errorf("expected final status 200 after redirect, got %d", redir.StatusCode)
	}
}

func TestEngineMaxPages(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	opts := DefaultOptions()
	opts.MaxDepth = 5
	opts.MaxPages = 2
	engine := New(opts, NewHTTPFetcher(opts))

	result, err := engine.Crawl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("crawl error: %v", err)
	}
	if len(result.Pages) > 2 {
		t.Errorf("MaxPages=2 not honored: crawled %d pages", len(result.Pages))
	}
}

func hostOf(raw string) string {
	const p = "://"
	i := indexOf(raw, p)
	if i < 0 {
		return ""
	}
	rest := raw[i+len(p):]
	for j := 0; j < len(rest); j++ {
		if rest[j] == '/' {
			return rest[:j]
		}
	}
	return rest
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
