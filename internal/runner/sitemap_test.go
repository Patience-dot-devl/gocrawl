package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// sitemapSite serves robots.txt declaring /sitemap.xml, a sitemap listing /, /a and /b, and a
// home page linking to /a, /b and /off (which is not in the sitemap). It counts how often the
// sitemap is fetched.
func sitemapSite(t *testing.T, sitemapHits *atomic.Int32) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", base)
		case "/sitemap.xml":
			sitemapHits.Add(1)
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>%[1]s/</loc></url><url><loc>%[1]s/a</loc></url><url><loc>%[1]s/b</loc></url></urlset>`, base)
		case "/":
			fmt.Fprint(w, `<html><head><title>Home</title></head><body><a href="/a">a</a><a href="/b">b</a><a href="/off">off</a></body></html>`)
		case "/a", "/b", "/off":
			fmt.Fprintf(w, `<html><head><title>%s</title></head><body>page</body></html>`, r.URL.Path)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestRunFetchesSitemapOnce(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	rep, err := Run(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("sitemap.xml fetched %d times, want exactly 1 (pre-pass shared with the analyzer)", got)
	}
	if rep.Coverage == nil || rep.Coverage.SitemapTotal != 3 || rep.Coverage.SitemapFetched != 3 {
		t.Fatalf("Coverage = %+v, want sitemap_total 3 and sitemap_fetched 3", rep.Coverage)
	}
	found := false
	for _, is := range rep.Issues {
		if is.Code == "sitemap-coverage" {
			found = true
		}
	}
	if !found {
		t.Error("the sitemap analyzer should still report sitemap-coverage from the pre-fetched set")
	}
}

func TestRunBudgetNoteOnlyWhenBudgetBelowSitemap(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	for _, tc := range []struct {
		maxPages int
		want     bool
	}{{2, true}, {3, false}, {0, false}} {
		cfg := config.Default()
		cfg.Crawl.MaxPages = tc.maxPages
		rep, err := Run(context.Background(), cfg, ts.URL)
		if err != nil {
			t.Fatalf("Run(max_pages=%d): %v", tc.maxPages, err)
		}
		has := false
		for _, n := range rep.Notes {
			if strings.HasPrefix(n, "The page budget") {
				has = true
			}
		}
		if has != tc.want {
			t.Errorf("max_pages=%d: budget note present=%v, want %v (notes %q)", tc.maxPages, has, tc.want, rep.Notes)
		}
	}
}

func TestRunWithoutSitemapLeavesCoverageSitemapFieldsZero(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `<html><head><title>Home</title></head><body>hi</body></html>`)
	}))
	defer ts.Close()
	rep, err := Run(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Coverage.SitemapTotal != 0 || rep.Coverage.SitemapFetched != 0 {
		t.Errorf("Coverage = %+v, want zero sitemap fields without a sitemap", rep.Coverage)
	}
	// No sitemap means no page can be off it: tape_off is left empty, not a run of '0'.
	if rep.Coverage.Tape != "2" || rep.Coverage.TapeOff != "" {
		t.Errorf("tape %q / %q, want 2 and an empty tape_off without a sitemap", rep.Coverage.Tape, rep.Coverage.TapeOff)
	}
}

func TestBudgetNote(t *testing.T) {
	want := "The page budget (500) is below the sitemap total (1,240); this crawl covered at most 40% of the sitemap."
	if got := budgetNote(500, 1240); got != want {
		t.Errorf("budgetNote(500, 1240) = %q, want %q", got, want)
	}
	for _, tc := range [][2]int{{0, 1240}, {1240, 1240}, {2000, 1240}, {10, 0}} {
		if got := budgetNote(tc[0], tc[1]); got != "" {
			t.Errorf("budgetNote(%d, %d) = %q, want no note", tc[0], tc[1], got)
		}
	}
	if got := budgetNote(1, 1000); !strings.Contains(got, "at most 1%") {
		t.Errorf("budgetNote(1, 1000) = %q, want the percentage floored at 1%%", got)
	}
}

func TestGroupThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1240: "1,240", 1234567: "1,234,567", -1240: "-1,240"} {
		if got := groupThousands(n); got != want {
			t.Errorf("groupThousands(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDiscoverSitemapSkipsWhenContextCanceled(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	cfg := config.Default()
	opts, err := cfg.ToOptions()
	if err != nil {
		t.Fatal(err)
	}
	f := crawler.NewHTTPFetcher(opts)
	engine := crawler.New(opts, f)
	seed, err := engine.SetSeed(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sm := discoverSitemap(ctx, engine, engine.Govern(f), seed)
	if sm.found() || len(sm.inScope) != 0 {
		t.Errorf("canceled pre-pass = %+v, want an empty set", sm)
	}
	if hits.Load() != 0 {
		t.Errorf("sitemap fetched %d times with a canceled context, want 0", hits.Load())
	}
}

// A sitemap that only lists URLs outside the crawl scope (a www. host for an apex seed, a CDN
// mirror) is found but counts no in-scope pages: Coverage has no sitemap totals, and every
// fetched page is "not in the sitemap" on the tape.
func TestRunCountsOnlyInScopeSitemapURLs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://www.other.example/</loc></url><url><loc>https://www.other.example/a</loc></url></urlset>`)
		case "/":
			fmt.Fprint(w, `<html><head><title>Home</title></head><body>hi</body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	rep, err := Run(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Coverage.SitemapTotal != 0 || rep.Coverage.SitemapFetched != 0 {
		t.Errorf("Coverage = %+v, want no in-scope sitemap URLs counted", rep.Coverage)
	}
	if rep.Coverage.Tape != "2" || rep.Coverage.TapeOff != "1" {
		t.Errorf("tape %q / %q, want 2 / 1 (a sitemap was found; the page is not in its in-scope part)", rep.Coverage.Tape, rep.Coverage.TapeOff)
	}
}

func TestPreviewSitemap(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)

	got, err := PreviewSitemap(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("PreviewSitemap: %v", err)
	}
	if !got.Found || got.Total != 3 || !got.ViaRobots {
		t.Errorf("preview = %+v, want found, 3 in scope, via robots.txt", got)
	}
	if len(got.Sources) != 1 || !strings.HasSuffix(got.Sources[0], "/sitemap.xml") {
		t.Errorf("Sources = %v, want the /sitemap.xml robots.txt names", got.Sources)
	}

	cfg := config.Default()
	cfg.Crawl.Exclude = []string{`/b$`}
	scoped, err := PreviewSitemap(context.Background(), cfg, ts.URL)
	if err != nil {
		t.Fatalf("PreviewSitemap with exclude: %v", err)
	}
	if scoped.Total != 2 {
		t.Errorf("Total with /b excluded = %d, want 2", scoped.Total)
	}
}

func TestPreviewSitemapWithoutSitemap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	defer ts.Close()
	got, err := PreviewSitemap(context.Background(), config.Default(), ts.URL)
	if err != nil {
		t.Fatalf("PreviewSitemap: %v", err)
	}
	if got.Found || got.Total != 0 || got.Sources == nil || len(got.Sources) != 0 {
		t.Errorf("preview = %+v, want not found with an empty (non-nil) source list", got)
	}
}

func TestPreviewSitemapRejectsBadRegex(t *testing.T) {
	cfg := config.Default()
	cfg.Crawl.Exclude = []string{"("}
	if _, err := PreviewSitemap(context.Background(), cfg, "https://example.com"); err == nil {
		t.Error("expected an error for an invalid exclude regex")
	}
}
