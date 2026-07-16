package robotscheck_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/robotscheck"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// TestUsesRotationPoolUserAgent guards against a real bug: robots.txt disallow checks were
// always tested against opts.UserAgent, even when a UserAgents rotation pool superseded it for
// actual requests, so a rule specific to the pool's real identity was silently never applied.
func TestUsesRotationPoolUserAgent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "User-agent: BotA\nDisallow: /secret\n\nUser-agent: *\nAllow: /\n")
	})
	mux.HandleFunc("/secret", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "shh")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	opts := crawler.DefaultOptions()
	opts.UserAgent = "gocrawl-default" // allowed by the "*" rule
	opts.UserAgents = []string{"BotA"} // supersedes UserAgent for real requests; disallowed
	opts.RespectRobots = false         // crawl /secret anyway so the analyzer has something to check

	engine := crawler.New(opts, crawler.NewHTTPFetcher(opts))
	result, err := engine.Crawl(context.Background(), ts.URL+"/secret")
	if err != nil {
		t.Fatalf("crawl error: %v", err)
	}

	found := false
	for _, iss := range robotscheck.New().Analyze(context.Background(), result) {
		if iss.Code == "robots-crawled-disallowed" {
			found = true
		}
	}
	if !found {
		t.Error("expected robots-crawled-disallowed when checked against the pool's actual UA (BotA), not the unused default")
	}
}
