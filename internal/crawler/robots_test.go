package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRobotsDataTestAgent(t *testing.T) {
	tests := []struct {
		name string
		data *RobotsData
		want bool
	}{
		{"nil receiver allows all", nil, true},
		{"zero status (fetch error) disallows", &RobotsData{Status: 0}, false},
		{"404 allows all", &RobotsData{Status: 404}, true},
		{"403 allows all", &RobotsData{Status: 403}, true},
		{"500 disallows", &RobotsData{Status: 500}, false},
		{"503 disallows", &RobotsData{Status: 503}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.TestAgent("/anything", "gocrawl"); got != tt.want {
				t.Errorf("TestAgent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRobotsManagerFetch(t *testing.T) {
	newManager := func(t *testing.T, status int, body string) (*robotsManager, *httptest.Server) {
		t.Helper()
		mux := http.NewServeMux()
		mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
			if status != 0 {
				w.WriteHeader(status)
			}
			if body != "" {
				_, _ = w.Write([]byte(body))
			}
		})
		ts := httptest.NewServer(mux)
		opts := DefaultOptions()
		return newRobotsManager(NewHTTPFetcher(opts), opts.UserAgent), ts
	}

	t.Run("200 with disallow rule blocks the path", func(t *testing.T) {
		mgr, ts := newManager(t, 200, "User-agent: *\nDisallow: /private\n")
		defer ts.Close()
		u, _ := url.Parse(ts.URL + "/private")
		if mgr.allowed(context.Background(), u) {
			t.Error("expected /private to be disallowed")
		}
		u2, _ := url.Parse(ts.URL + "/public")
		if !mgr.allowed(context.Background(), u2) {
			t.Error("expected /public to be allowed")
		}
	})

	t.Run("404 allows everything", func(t *testing.T) {
		mgr, ts := newManager(t, 404, "")
		defer ts.Close()
		u, _ := url.Parse(ts.URL + "/anything")
		if !mgr.allowed(context.Background(), u) {
			t.Error("expected 404 robots.txt to allow all")
		}
	})

	t.Run("500 disallows everything", func(t *testing.T) {
		mgr, ts := newManager(t, 500, "")
		defer ts.Close()
		u, _ := url.Parse(ts.URL + "/anything")
		if mgr.allowed(context.Background(), u) {
			t.Error("expected 500 robots.txt to disallow all")
		}
	})

	t.Run("empty 200 body allows everything", func(t *testing.T) {
		mgr, ts := newManager(t, 200, "")
		defer ts.Close()
		u, _ := url.Parse(ts.URL + "/anything")
		if !mgr.allowed(context.Background(), u) {
			t.Error("expected empty-body robots.txt to allow all")
		}
	})

	t.Run("unparseable 200 body allows everything", func(t *testing.T) {
		mgr, ts := newManager(t, 200, "\x00\x01\x02not valid robots.txt at all \xff\xfe")
		defer ts.Close()
		u, _ := url.Parse(ts.URL + "/anything")
		if !mgr.allowed(context.Background(), u) {
			t.Error("expected unparseable robots.txt to allow all")
		}
	})

	t.Run("connection failure disallows everything", func(t *testing.T) {
		opts := DefaultOptions()
		mgr := newRobotsManager(NewHTTPFetcher(opts), opts.UserAgent)
		// Port 0 host that nothing listens on; use a closed server instead so the
		// connection is refused rather than merely never dialed.
		ts := httptest.NewServer(http.NewServeMux())
		ts.Close()
		u, _ := url.Parse(ts.URL + "/anything")
		if mgr.allowed(context.Background(), u) {
			t.Error("expected a fetch/connection error to disallow all")
		}
	})

	t.Run("result is cached across calls", func(t *testing.T) {
		var hits int
		mux := http.NewServeMux()
		mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
			hits++
			w.WriteHeader(500)
		})
		ts := httptest.NewServer(mux)
		defer ts.Close()
		opts := DefaultOptions()
		mgr := newRobotsManager(NewHTTPFetcher(opts), opts.UserAgent)
		u, _ := url.Parse(ts.URL + "/a")
		u2, _ := url.Parse(ts.URL + "/b")
		if mgr.allowed(context.Background(), u) {
			t.Error("expected disallow on first call")
		}
		if mgr.allowed(context.Background(), u2) {
			t.Error("expected disallow on second call (cached)")
		}
		if hits != 1 {
			t.Errorf("expected robots.txt to be fetched once (cached), got %d fetches", hits)
		}
	})
}
