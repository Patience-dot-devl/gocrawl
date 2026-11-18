package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// reg builds the default registry. The fetcher is only used by analyzers at run time, not
// during selection, so a plain HTTP fetcher is fine here.
func reg() *analyze.Registry {
	return BuildRegistry(crawler.NewHTTPFetcher(crawler.DefaultOptions()), RegistryOptions{})
}

func nameSet(as []analyze.Analyzer) map[string]bool { return names(as) }

func TestPlanAnalyzersStripQueryOff(t *testing.T) {
	got, skipped := planAnalyzers(reg(), config.AnalyzersConfig{}, false)
	if skipped != nil {
		t.Errorf("expected no skips with strip_query off, got %v", skipped)
	}
	for _, n := range queryDependentAnalyzers {
		if !nameSet(got)[n] {
			t.Errorf("query-dependent analyzer %q should run when strip_query is off", n)
		}
	}
}

func TestPlanAnalyzersStripQuerySkipsQueryAnalyzers(t *testing.T) {
	got, skipped := planAnalyzers(reg(), config.AnalyzersConfig{}, true)

	active := nameSet(got)
	for _, n := range queryDependentAnalyzers {
		if active[n] {
			t.Errorf("analyzer %q should be skipped when strip_query is on", n)
		}
	}
	// Unrelated analyzers still run.
	if !active["seo"] {
		t.Error("seo should still run when strip_query is on")
	}

	sort.Strings(skipped)
	want := append([]string{}, queryDependentAnalyzers...)
	sort.Strings(want)
	if len(skipped) != len(want) {
		t.Fatalf("skipped = %v, want %v", skipped, want)
	}
	for i := range want {
		if skipped[i] != want[i] {
			t.Fatalf("skipped = %v, want %v", skipped, want)
		}
	}
}

func TestPlanAnalyzersStripQueryOnlyReportsActiveSkips(t *testing.T) {
	// Allow-list excludes the query-dependent analyzers, so strip_query skips nothing.
	cfg := config.AnalyzersConfig{Enabled: []string{"seo", "links"}}
	got, skipped := planAnalyzers(reg(), cfg, true)
	if skipped != nil {
		t.Errorf("expected no skips when query-dependent analyzers are not selected, got %v", skipped)
	}
	if active := nameSet(got); !active["seo"] || !active["links"] {
		t.Errorf("expected seo and links to run, got %v", active)
	}
}

func TestPlanAnalyzersStripQuerySkipsExplicitlyEnabled(t *testing.T) {
	// A query-dependent analyzer explicitly enabled is still skipped under strip_query.
	cfg := config.AnalyzersConfig{Enabled: []string{"seo", "utm"}}
	got, skipped := planAnalyzers(reg(), cfg, true)
	if nameSet(got)["utm"] {
		t.Error("utm should be skipped under strip_query even when explicitly enabled")
	}
	if len(skipped) != 1 || skipped[0] != "utm" {
		t.Errorf("expected skipped=[utm], got %v", skipped)
	}
}

func TestCoverageNote(t *testing.T) {
	// Page limit → message names --max-pages and warns about incomplete findings.
	n := coverageNote(crawler.Coverage{DiscoveredNotCrawled: 12, PageLimitReached: true, MaxPages: 100})
	for _, want := range []string{"partial coverage", "12 in-scope", "--max-pages 100", "broken links"} {
		if !strings.Contains(n, want) {
			t.Errorf("page-limit note missing %q: %s", want, n)
		}
	}
	// Depth limit → message names --depth.
	if d := coverageNote(crawler.Coverage{DiscoveredNotCrawled: 3, DepthLimitReached: true, MaxDepth: 2}); !strings.Contains(d, "--depth 2") {
		t.Errorf("depth-limit note missing --depth: %s", d)
	}
	// Interrupted (e.g. Ctrl-C) → distinct message, not the limit-reached wording.
	i := coverageNote(crawler.Coverage{Interrupted: true})
	if !strings.Contains(i, "interrupted") {
		t.Errorf("interrupted note missing \"interrupted\": %s", i)
	}
	if strings.Contains(i, "--max-pages") || strings.Contains(i, "--depth") {
		t.Errorf("interrupted note shouldn't mention a limit flag: %s", i)
	}
	// Duration limit (--max-duration) → distinct message, takes priority over the generic
	// Interrupted wording even though DurationLimitReached implies Interrupted.
	d := coverageNote(crawler.Coverage{Interrupted: true, DurationLimitReached: true})
	if !strings.Contains(d, "--max-duration") {
		t.Errorf("duration-limit note missing --max-duration: %s", d)
	}
	if strings.Contains(d, "Ctrl-C") {
		t.Errorf("duration-limit note shouldn't use the generic Ctrl-C wording: %s", d)
	}
}

// TestRunDoesNotLeakBasicAuthToSitemapHost is an end-to-end guard for a real credential leak:
// the sitemap analyzer fetches whatever URL robots.txt's Sitemap: directive names — routinely
// a different host (a CDN, a separate subdomain) — via a fetcher Run builds fresh for the
// analyzer registry (runner.go, "Sitemap analyzer fetches with a raw fetcher"). That fetcher
// must not carry the seed's Basic Auth to that host. Unlike the FollowExternal leak this
// guards against no config flag beyond basic_auth; a sitemap hosted off the seed's own host is
// an entirely ordinary site, not an edge case.
func TestRunDoesNotLeakBasicAuthToSitemapHost(t *testing.T) {
	var sitemapAuth, seedAuth string

	sitemapHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sitemapAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`)
	}))
	defer sitemapHost.Close()

	// "localhost" rather than "127.0.0.1" so it's a genuinely different host string from the
	// seed's (sameSite strips ports before comparing, so two httptest servers on 127.0.0.1 at
	// different ports would otherwise be treated as the same site).
	sitemapURL := strings.Replace(sitemapHost.URL, "127.0.0.1", "localhost", 1) + "/sitemap.xml"

	seed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			seedAuth = r.Header.Get("Authorization")
			fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s\n", sitemapURL)
		default:
			fmt.Fprint(w, "<html><head><title>Home</title></head><body>hello</body></html>")
		}
	}))
	defer seed.Close()

	cfg := config.Default()
	cfg.Crawl.BasicAuth = "alice:s3cret"
	cfg.Crawl.MaxDepth = 0
	cfg.Crawl.MaxPages = 5

	if _, err := Run(context.Background(), cfg, seed.URL); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if seedAuth == "" {
		t.Error("expected the seed host's robots.txt fetch to carry Basic Auth")
	}
	if sitemapAuth != "" {
		t.Errorf("Authorization = %q, want empty (credentials leaked to the sitemap's host)", sitemapAuth)
	}
}

// TestRunScopesCustomHeadersForAnalyzerFetches: the analyzer registry's fetcher must send the
// custom headers to the seed host (a Shopify store's sitemap sits behind the same bot
// protection as its pages) but not to a sitemap hosted elsewhere.
func TestRunScopesCustomHeadersForAnalyzerFetches(t *testing.T) {
	var mu sync.Mutex
	var offHostSig string
	seedSigs := map[string]string{}

	sitemapHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		offHostSig = r.Header.Get("Signature")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`)
	}))
	defer sitemapHost.Close()
	offHostSitemap := strings.Replace(sitemapHost.URL, "127.0.0.1", "localhost", 1) + "/sitemap.xml"

	var seedURL string
	seed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seedSigs[r.URL.Path] = r.Header.Get("Signature")
		mu.Unlock()
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\nSitemap: %s\n", seedURL, offHostSitemap)
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`)
		default:
			fmt.Fprint(w, "<html><head><title>Home</title></head><body>hello</body></html>")
		}
	}))
	defer seed.Close()
	seedURL = seed.URL

	cfg := config.Default()
	cfg.Crawl.Headers = []string{"Signature: sig1=:abc:"}
	cfg.Crawl.MaxPages = 5

	if _, err := Run(context.Background(), cfg, seed.URL); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"/", "/robots.txt", "/sitemap.xml"} {
		if got := seedSigs[path]; got != "sig1=:abc:" {
			t.Errorf("seed %s Signature = %q, want sig1=:abc:", path, got)
		}
	}
	if offHostSig != "" {
		t.Errorf("off-host sitemap Signature = %q, want empty (header leaked to another host)", offHostSig)
	}
}

// TestRunAnalyzerProbesRespectRobots: the fetcher Run hands to the analyzer registry drives
// every extra request an analyzer makes (sitemap.xml, llms.txt, the --specialized WordPress
// and Shopify probes). Those must obey the same robots.txt policy as the crawl, or a site
// that disallows /wp-json/ still gets probed there.
func TestRunAnalyzerProbesRespectRobots(t *testing.T) {
	var probed []string
	var mu sync.Mutex
	seed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprint(w, "User-agent: *\nDisallow: /sitemap.xml\nDisallow: /llms.txt\n")
		case "/sitemap.xml", "/llms.txt":
			mu.Lock()
			probed = append(probed, r.URL.Path)
			mu.Unlock()
			http.NotFound(w, r)
		default:
			fmt.Fprint(w, "<html><head><title>Home</title></head><body>hello</body></html>")
		}
	}))
	defer seed.Close()

	cfg := config.Default()
	cfg.Crawl.MaxDepth = 0
	cfg.Crawl.MaxPages = 5
	cfg.Crawl.RespectRobots = true

	if _, err := Run(context.Background(), cfg, seed.URL); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(probed) != 0 {
		t.Fatalf("analyzers fetched robots-disallowed paths %v", probed)
	}
}

// TestRunRollsUpSiteWideInfo crawls a 12-page site whose template lacks a viewport meta tag.
// By default the finding is reported once for the host; with output.per_page_info it is
// listed on every page.
func TestRunRollsUpSiteWideInfo(t *testing.T) {
	const pages = 12
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<html><head><title>Page %s of the test site</title></head><body><h1>Hi</h1>`, r.URL.Path)
		for i := range pages - 1 {
			fmt.Fprintf(w, `<a href="/p%d">p</a>`, i)
		}
		fmt.Fprint(w, `</body></html>`)
	}))
	defer site.Close()

	count := func(perPage bool) (viewport int, rolled bool) {
		t.Helper()
		cfg := config.Default()
		cfg.Crawl.MaxDepth = 0
		cfg.Crawl.MaxPages = pages
		cfg.Analyzers.Enabled = []string{"seo"}
		cfg.Output.PerPageInfo = perPage
		rep, err := Run(context.Background(), cfg, site.URL)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		for _, is := range rep.Issues {
			if is.Code == "seo-missing-viewport" {
				viewport++
				rolled = rolled || analyze.IsSiteWide(is)
			}
		}
		return viewport, rolled
	}

	if n, rolled := count(false); n != 1 || !rolled {
		t.Errorf("default: %d seo-missing-viewport issues (rolled up: %v), want 1 rolled-up issue", n, rolled)
	}
	if n, rolled := count(true); n != pages || rolled {
		t.Errorf("per_page_info: %d seo-missing-viewport issues (rolled up: %v), want %d per-page issues", n, rolled, pages)
	}
}

// listSite serves /a and /b (both in its sitemap) linking to /c and /d (not in it), and
// records every page path fetched.
func listSite(t *testing.T, withSitemap bool) (*httptest.Server, func() []string) {
	t.Helper()
	var (
		mu    sync.Mutex
		paths []string
		base  string
	)
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			http.NotFound(w, r)
		case "/sitemap.xml":
			if !withSitemap {
				http.NotFound(w, r)
				return
			}
			fmt.Fprintf(w, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>%s/a</loc></url><url><loc>%s/b</loc></url></urlset>`, base, base)
		default:
			if strings.HasSuffix(r.URL.Path, ".xml") { // sitemap discovery probes other paths
				http.NotFound(w, r)
				return
			}
			mu.Lock()
			paths = append(paths, r.URL.Path)
			mu.Unlock()
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><head><title>List mode test page</title></head><body><a href="/c">c</a><a href="/d">d</a></body></html>`)
		}
	}))
	base = site.URL
	t.Cleanup(site.Close)
	return site, func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := append([]string(nil), paths...)
		sort.Strings(out)
		return out
	}
}

func TestRunListModeCrawlsOnlyListedURLs(t *testing.T) {
	site, fetched := listSite(t, false)
	cfg := config.Default()
	cfg.Analyzers.Enabled = []string{"seo"}
	cfg.Crawl.URLs = []string{site.URL + "/a", "https://other.example/x"}

	rep, err := Run(context.Background(), cfg, site.URL+"/a")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := fetched(); len(got) != 1 || got[0] != "/a" {
		t.Errorf("fetched %v, want only /a (its links not followed, the other host skipped)", got)
	}
	if !strings.Contains(strings.Join(rep.Notes, "\n"), "1 of 2 listed URLs were not crawled") {
		t.Errorf("notes %q do not report the skipped URL", rep.Notes)
	}
}

func TestRunFromSitemapCrawlsSitemapURLs(t *testing.T) {
	site, fetched := listSite(t, true)
	cfg := config.Default()
	cfg.Analyzers.Enabled = []string{"seo"}
	cfg.Crawl.FromSitemap = true

	rep, err := Run(context.Background(), cfg, site.URL)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := fetched(); strings.Join(got, ",") != "/a,/b" {
		t.Errorf("fetched %v, want /a and /b from the sitemap, and not the seed or /c and /d", got)
	}
	if !strings.Contains(strings.Join(rep.Notes, "\n"), "crawled the 2 listed URLs") {
		t.Errorf("notes = %q", rep.Notes)
	}

	nositemap, _ := listSite(t, false)
	if _, err := Run(context.Background(), cfg, nositemap.URL); err == nil || !strings.Contains(err.Error(), "from_sitemap") {
		t.Errorf("from_sitemap without a sitemap: err = %v, want a from_sitemap error", err)
	}
}
