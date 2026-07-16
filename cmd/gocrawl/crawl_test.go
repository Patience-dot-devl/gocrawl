package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newBasicAuthServer(t *testing.T, user, pass string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="gocrawl"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `<html><head><title>Authenticated Home Page</title></head><body>ok</body></html>`)
	})
	return httptest.NewServer(mux)
}

// A seed URL with embedded credentials (https://user:pass@host) must authenticate the crawl
// via Basic Auth, yet never appear with those credentials in the resulting report.
func TestCrawlStripsSeedCredentials(t *testing.T) {
	ts := newBasicAuthServer(t, "alice", "s3cret")
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	seedWithCreds := "http://alice:s3cret@" + host

	out := filepath.Join(t.TempDir(), "report.json")
	cmd := newCrawlCmd()
	if err := cmd.Flags().Set("format", "json"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("out", out); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("depth", "0"); err != nil {
		t.Fatal(err)
	}

	// runCrawl reads cmd.Context(), which cobra only populates via Execute()/ExecuteContext();
	// calling runCrawl directly (bypassing that) needs it set explicitly.
	cmd.SetContext(context.Background())
	if err := runCrawl(cmd, []string{seedWithCreds}); err != nil {
		t.Fatalf("runCrawl: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	if strings.Contains(string(data), "alice") || strings.Contains(string(data), "s3cret") {
		t.Errorf("report contains leaked credentials: %s", data)
	}

	var rep struct {
		Seed         string `json:"seed"`
		PagesCrawled int    `json:"pages_crawled"`
	}
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if strings.Contains(rep.Seed, "alice") || strings.Contains(rep.Seed, "@") {
		t.Errorf("report seed still has credentials: %q", rep.Seed)
	}
	if rep.PagesCrawled != 1 {
		t.Errorf("expected the Basic-Auth-gated page to be crawled successfully, pages_crawled=%d", rep.PagesCrawled)
	}
}

// An explicit --basic-auth flag takes precedence over credentials embedded in the seed URL.
func TestCrawlExplicitBasicAuthWinsOverSeedCredentials(t *testing.T) {
	ts := newBasicAuthServer(t, "realuser", "realpass")
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	seedWithCreds := "http://wronguser:wrongpass@" + host

	out := filepath.Join(t.TempDir(), "report.json")
	cmd := newCrawlCmd()
	if err := cmd.Flags().Set("format", "json"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("out", out); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("basic-auth", "realuser:realpass"); err != nil {
		t.Fatal(err)
	}

	// runCrawl reads cmd.Context(), which cobra only populates via Execute()/ExecuteContext();
	// calling runCrawl directly (bypassing that) needs it set explicitly.
	cmd.SetContext(context.Background())
	if err := runCrawl(cmd, []string{seedWithCreds}); err != nil {
		t.Fatalf("runCrawl: %v", err)
	}

	var rep struct {
		PagesCrawled int `json:"pages_crawled"`
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if rep.PagesCrawled != 1 {
		t.Errorf("expected the explicit --basic-auth credentials to authenticate the crawl, pages_crawled=%d", rep.PagesCrawled)
	}
}

// TestCrawlMaxDurationStopsEarly guards against the --max-duration flag never reaching the
// crawl: a home page linking to a page that hangs past the budget must still produce a report,
// labeled as partial coverage, rather than hanging or erroring.
func TestCrawlMaxDurationStopsEarly(t *testing.T) {
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home</title></head><body><a href="/slow">slow</a></body></html>`)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		<-release
		fmt.Fprint(w, `<html><body>too late</body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer func() {
		close(release)
		ts.Close()
	}()

	out := filepath.Join(t.TempDir(), "report.json")
	cmd := newCrawlCmd()
	for flag, value := range map[string]string{
		"format":       "json",
		"out":          out,
		"max-duration": "50ms",
	} {
		if err := cmd.Flags().Set(flag, value); err != nil {
			t.Fatal(err)
		}
	}
	cmd.SetContext(context.Background())
	if err := runCrawl(cmd, []string{ts.URL}); err != nil {
		t.Fatalf("runCrawl: %v", err)
	}

	var rep struct {
		Coverage struct {
			Complete             bool `json:"complete"`
			DurationLimitReached bool `json:"duration_limit_reached"`
		} `json:"coverage"`
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if !rep.Coverage.DurationLimitReached {
		t.Error("expected coverage.duration_limit_reached = true")
	}
	if rep.Coverage.Complete {
		t.Error("expected coverage.complete = false")
	}
}

// TestCrawlSiteMapTitleSurvivesBodyRelease guards against a real ordering bug: page bodies and
// parsed DOMs are released once nothing needs them anymore (see crawler.Result.ReleaseBodies),
// but the site-map tree's per-page title is extracted from a page's parsed Doc inside
// report.Build itself — releasing bodies even slightly too early would silently blank every
// title in the report.
func TestCrawlSiteMapTitleSurvivesBodyRelease(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home Sweet Home</title></head><body>ok</body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	out := filepath.Join(t.TempDir(), "report.json")
	cmd := newCrawlCmd()
	if err := cmd.Flags().Set("format", "json"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("out", out); err != nil {
		t.Fatal(err)
	}
	cmd.SetContext(context.Background())
	if err := runCrawl(cmd, []string{ts.URL}); err != nil {
		t.Fatalf("runCrawl: %v", err)
	}

	var rep struct {
		SiteMap struct {
			Root struct {
				Title string `json:"title"`
			} `json:"root"`
		} `json:"site_map"`
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if rep.SiteMap.Root.Title != "Home Sweet Home" {
		t.Errorf("site_map.root.title = %q, want %q", rep.SiteMap.Root.Title, "Home Sweet Home")
	}
}
