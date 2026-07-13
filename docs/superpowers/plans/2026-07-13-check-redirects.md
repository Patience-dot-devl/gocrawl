# check-redirects Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `gocrawl check-redirects` CLI subcommand that verifies a HubSpot-format redirect-rule CSV export against a live site, writing the CSV back out with appended columns describing source-redirect behavior, target liveness, and sitemap cross-checks.

**Architecture:** A new `internal/redirectcheck` package parses the CSV, classifies each rule by domain scope, fetches each in-scope rule's Original/Target URLs via the existing `crawler.HTTPFetcher` (no crawl engine involved — single-URL fetches only), cross-references a `sitemap.xml` fetched via a small exported helper extracted from `internal/analyze/sitemap`, and writes an annotated CSV. A thin `cmd/gocrawl/checkredirects.go` Cobra command wires flags to this package.

**Tech Stack:** Go 1.26, `encoding/csv`, `encoding/xml`, existing `crawler.Fetcher`/`HTTPFetcher`, `golang.org/x/time/rate` (already a dependency), `github.com/spf13/cobra`.

## Global Constraints

- Module: `github.com/Patience-dot-devl/gocrawl`, Go 1.26+.
- Run `gofmt` and `go vet ./...` before considering any task done; the project's CI also runs `go test -race` and `golangci-lint` (`errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `misspell`).
- No comments except where a non-obvious WHY needs explaining (see CLAUDE.md); never explain WHAT the code does.
- Analyzers/packages must stay side-effect free and testable via fake `crawler.Fetcher` implementations — this feature follows the same convention even though it isn't an `analyze.Analyzer`.
- Every new package/file needs tests; no live-network tests — everything driven through a fake fetcher, except the CLI-level tests, which must stay hermetic (fail before any network call).
- Spec: `docs/superpowers/specs/2026-07-13-check-redirects-design.md` — consult it for the full verdict-priority rationale if anything below seems ambiguous.

---

### Task 1: Extract an exported `sitemap.Fetch` helper

**Files:**
- Modify: `internal/analyze/sitemap/sitemap.go`
- Test: `internal/analyze/sitemap/sitemap_test.go`

**Interfaces:**
- Produces: `func Fetch(ctx context.Context, fetcher crawler.Fetcher, candidates map[string]bool) (urls map[string]bool, parsed int, invalidDeclared []string)` — exported from package `sitemap`. `candidates` maps a sitemap URL to whether it was *declared* (found via robots.txt or a parent sitemap index) as opposed to merely guessed at a conventional path. `urls` is every `<loc>` found (normalized, trailing slash stripped except root, `#fragment` dropped), `parsed` counts candidates that successfully parsed as an urlset or sitemap index (0 means nothing usable was found anywhere), `invalidDeclared` lists declared candidates whose response didn't parse as either.

- [ ] **Step 1: Write a failing test that calls the not-yet-existing `Fetch` directly**

Add to `internal/analyze/sitemap/sitemap_test.go` (this file already has `fakeFetcher` and the `validSitemap` const — reuse both):

```go
func TestFetchDirect(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(validSitemap)},
	}}
	urls, parsed, invalid := sitemap.Fetch(context.Background(), ff, map[string]bool{"https://example.com/sitemap.xml": false})
	if parsed != 1 {
		t.Fatalf("parsed = %d, want 1", parsed)
	}
	if len(invalid) != 0 {
		t.Fatalf("invalidDeclared = %v, want empty", invalid)
	}
	want := []string{"https://example.com/a", "https://example.com/b"}
	if len(urls) != len(want) {
		t.Fatalf("got %d urls, want %d: %v", len(urls), len(want), urls)
	}
	for _, u := range want {
		if !urls[u] {
			t.Errorf("missing url %q in result %v", u, urls)
		}
	}
}

func TestFetchDirectFlagsDeclaredInvalid(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/broken.xml": {StatusCode: 200, ContentType: "text/html", Body: []byte(softHTML)},
	}}
	_, parsed, invalid := sitemap.Fetch(context.Background(), ff, map[string]bool{"https://example.com/broken.xml": true})
	if parsed != 0 {
		t.Fatalf("parsed = %d, want 0", parsed)
	}
	if len(invalid) != 1 || invalid[0] != "https://example.com/broken.xml" {
		t.Fatalf("invalidDeclared = %v, want [https://example.com/broken.xml]", invalid)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail on the undefined symbol**

Run: `go test ./internal/analyze/sitemap/... -run TestFetchDirect -v`
Expected: FAIL — `undefined: sitemap.Fetch`

- [ ] **Step 3: Extract `Fetch` and rewrite `Analyzer.Analyze` to use it**

Replace the full contents of `internal/analyze/sitemap/sitemap.go` with:

```go
// Package sitemap discovers and parses sitemap.xml (including sitemap indexes) and
// cross-checks declared URLs against what was actually crawled.
package sitemap

import (
	"context"
	"encoding/xml"
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer fetches and validates sitemaps. It uses a Fetcher to retrieve sitemap files.
type Analyzer struct {
	fetcher crawler.Fetcher
}

// New returns a sitemap analyzer that fetches sitemaps with the given fetcher.
func New(fetcher crawler.Fetcher) *Analyzer { return &Analyzer{fetcher: fetcher} }

func (Analyzer) Name() string { return "sitemap" }
func (Analyzer) Description() string {
	return "sitemap.xml discovery/parsing (incl. index) and crawl-coverage cross-check"
}

type urlset struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

type sitemapindex struct {
	XMLName  xml.Name `xml:"sitemapindex"`
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

// Fetch retrieves and parses the sitemap(s) reachable from candidates (URL -> declared, where
// declared is true for sitemaps named in robots.txt or a parent index, false for a
// conventional path merely guessed at). It follows sitemap indexes up to two levels deep.
// parsed counts candidates that successfully parsed as an urlset or index (0 means nothing
// usable was found at any candidate); invalidDeclared lists declared candidates whose response
// parsed as neither (a guessed path that fails to parse is almost always a soft-404 and isn't
// reported).
func Fetch(ctx context.Context, fetcher crawler.Fetcher, candidates map[string]bool) (urls map[string]bool, parsed int, invalidDeclared []string) {
	urls = map[string]bool{}
	visited := map[string]bool{}

	var fetchOne func(smURL string, depth int, declared bool)
	fetchOne = func(smURL string, depth int, declared bool) {
		if depth > 2 || visited[smURL] {
			return
		}
		visited[smURL] = true

		page, ferr := fetcher.Fetch(ctx, smURL)
		if ferr != nil || page == nil || page.StatusCode != 200 || len(page.Body) == 0 {
			return
		}

		var idx sitemapindex
		if xml.Unmarshal(page.Body, &idx) == nil && len(idx.Sitemaps) > 0 {
			parsed++
			for _, s := range idx.Sitemaps {
				if loc := strings.TrimSpace(s.Loc); loc != "" {
					fetchOne(loc, depth+1, true)
				}
			}
			return
		}
		var us urlset
		if xml.Unmarshal(page.Body, &us) == nil {
			parsed++
			for _, u := range us.URLs {
				if loc := strings.TrimSpace(u.Loc); loc != "" {
					urls[normalize(loc)] = true
				}
			}
			return
		}
		if declared {
			invalidDeclared = append(invalidDeclared, smURL)
		}
	}

	for c, declared := range candidates {
		fetchOne(c, 0, declared)
	}
	return urls, parsed, invalidDeclared
}

func (a Analyzer) Analyze(ctx context.Context, result *crawler.Result) []analyze.Issue {
	seed, err := url.Parse(result.Seed)
	if err != nil {
		return nil
	}
	var issues []analyze.Issue

	// Candidate sitemap URLs: those declared in robots.txt (declared==true) plus common
	// conventional paths we merely guess at (declared==false). The distinction matters for
	// error reporting: a declared sitemap that won't parse is a real misconfiguration worth
	// flagging, but a guessed path that returns non-XML is almost always a soft-404 (the
	// server answers 200 with an HTML page for any unknown path), so we stay silent about it.
	candidates := map[string]bool{}
	for _, data := range result.Robots {
		for _, sm := range data.Sitemaps {
			candidates[sm] = true
		}
	}
	base := seed.Scheme + "://" + seed.Host
	for _, path := range []string{"/sitemap.xml", "/sitemap_index.xml"} {
		if _, ok := candidates[base+path]; !ok {
			candidates[base+path] = false
		}
	}

	sitemapURLs, parsed, invalidDeclared := Fetch(ctx, a.fetcher, candidates)
	for _, u := range invalidDeclared {
		issues = append(issues, analyze.Issue{
			Analyzer: "sitemap", URL: u, Severity: analyze.Warning,
			Code: "sitemap-invalid", Message: "Could not parse sitemap as urlset or index",
		})
	}

	if parsed == 0 {
		issues = append(issues, analyze.Issue{
			Analyzer: "sitemap", URL: base, Severity: analyze.Warning,
			Code: "sitemap-missing", Message: "No sitemap found at robots.txt or conventional locations",
		})
		return issues
	}

	// Cross-check coverage.
	crawled := map[string]bool{}
	for _, p := range result.Pages {
		if p.StatusCode == 200 {
			crawled[normalize(p.FinalURL)] = true
		}
	}

	var notInSitemap, notCrawled int
	for u := range crawled {
		if !sitemapURLs[u] {
			notInSitemap++
		}
	}
	for u := range sitemapURLs {
		if !crawled[u] {
			notCrawled++
		}
	}

	issues = append(issues, analyze.Issue{
		Analyzer: "sitemap", URL: base, Severity: analyze.Info,
		Code: "sitemap-coverage", Message: "Sitemap vs. crawl coverage",
		Data: map[string]any{
			"sitemap_urls":           len(sitemapURLs),
			"crawled_pages":          len(crawled),
			"crawled_not_in_sitemap": notInSitemap,
			"in_sitemap_not_crawled": notCrawled,
		},
	})
	return issues
}

func normalize(u string) string {
	u = strings.TrimSpace(u)
	if i := strings.Index(u, "#"); i >= 0 {
		u = u[:i]
	}
	if len(u) > 1 {
		u = strings.TrimRight(u, "/")
	}
	return u
}
```

- [ ] **Step 4: Run all sitemap tests to verify everything passes**

Run: `go test ./internal/analyze/sitemap/... -v`
Expected: PASS — all of `TestGuessedIndexSoft404NotFlagged`, `TestDeclaredBrokenSitemapFlagged`, `TestNoSitemapWhenOnlySoft404s`, `TestFetchDirect`, `TestFetchDirectFlagsDeclaredInvalid`.

- [ ] **Step 5: Commit**

```bash
git add internal/analyze/sitemap/sitemap.go internal/analyze/sitemap/sitemap_test.go
git commit -m "Extract exported sitemap.Fetch helper for reuse outside the analyzer"
```

---

### Task 2: `redirectcheck` package skeleton — `Rule` type and CSV parsing

**Files:**
- Create: `internal/redirectcheck/parse.go`
- Test: `internal/redirectcheck/parse_test.go`

**Interfaces:**
- Produces: `type Rule struct { Original, Target, RedirectType, RedirectStyle, Priority, MatchQueryStrings, IgnoreTrailingSlash, IgnoreProtocol, DisableIfPageExists, Note string }`; `func ParseCSV(r io.Reader) ([]Rule, error)`.

- [ ] **Step 1: Write the failing test**

Create `internal/redirectcheck/parse_test.go`:

```go
package redirectcheck_test

import (
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

const sampleCSV = `"Original URL","Redirect to","Redirect type","Redirect style","Priority","Match query strings","Ignore trailing slash","Ignore protocol","Disable if page exists","Note"
"/old-page","/new-page","STANDARD","301","1000000001","FALSE","TRUE","TRUE","TRUE",""
"https?://example.com/cases(?P<page_slug>/.*)?$","https://example.com/en/cases{page_slug}","STANDARD","301","2000000000","FALSE","TRUE","TRUE","TRUE",""
`

func TestParseCSV(t *testing.T) {
	rules, err := redirectcheck.ParseCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	if rules[0].Original != "/old-page" || rules[0].Target != "/new-page" {
		t.Errorf("row 0 = %+v, want Original=/old-page Target=/new-page", rules[0])
	}
	if rules[0].DisableIfPageExists != "TRUE" {
		t.Errorf("row 0 DisableIfPageExists = %q, want TRUE", rules[0].DisableIfPageExists)
	}
	if !strings.Contains(rules[1].Original, "(?P<") {
		t.Errorf("row 1 Original should retain regex syntax, got %q", rules[1].Original)
	}
}

func TestParseCSVRejectsWrongSchema(t *testing.T) {
	bad := "\"Original URL\",\"Redirect to\"\n\"/a\",\"/b\"\n"
	if _, err := redirectcheck.ParseCSV(strings.NewReader(bad)); err == nil {
		t.Fatal("expected an error for a CSV with the wrong column schema")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/redirectcheck/... -v`
Expected: FAIL — `no Go files in internal/redirectcheck` (or `undefined: redirectcheck.ParseCSV` once the package exists).

- [ ] **Step 3: Write the implementation**

Create `internal/redirectcheck/parse.go`:

```go
// Package redirectcheck verifies a redirect-rule CSV export (HubSpot's URL Redirects tool
// schema) against a live site: whether each rule's source still redirects correctly, whether
// its target is a live page, and whether both sides agree with the site's current sitemap.xml.
package redirectcheck

import (
	"encoding/csv"
	"fmt"
	"io"
)

// expectedHeader is the exact HubSpot URL Redirects export column order.
var expectedHeader = []string{
	"Original URL", "Redirect to", "Redirect type", "Redirect style", "Priority",
	"Match query strings", "Ignore trailing slash", "Ignore protocol", "Disable if page exists", "Note",
}

// Rule is one row of the redirect-rule CSV. Columns are kept as raw strings so they can be
// echoed back verbatim in the output report.
type Rule struct {
	Original            string
	Target              string
	RedirectType        string
	RedirectStyle       string
	Priority            string
	MatchQueryStrings   string
	IgnoreTrailingSlash string
	IgnoreProtocol      string
	DisableIfPageExists string
	Note                string
}

// ParseCSV reads a HubSpot-format redirect-rule export. It returns an error if the header
// doesn't match the expected column schema.
func ParseCSV(r io.Reader) ([]Rule, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	if !equalStrings(header, expectedHeader) {
		return nil, fmt.Errorf("unexpected CSV columns\n got:  %v\n want: %v", header, expectedHeader)
	}

	var rules []Rule
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV row: %w", err)
		}
		rules = append(rules, Rule{
			Original:            record[0],
			Target:              record[1],
			RedirectType:        record[2],
			RedirectStyle:       record[3],
			Priority:            record[4],
			MatchQueryStrings:   record[5],
			IgnoreTrailingSlash: record[6],
			IgnoreProtocol:      record[7],
			DisableIfPageExists: record[8],
			Note:                record[9],
		})
	}
	return rules, nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/redirectcheck/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/redirectcheck/parse.go internal/redirectcheck/parse_test.go
git commit -m "Add redirectcheck.ParseCSV for HubSpot redirect-rule exports"
```

---

### Task 3: Domain-scope classification and URL helpers

**Files:**
- Create: `internal/redirectcheck/scope.go`
- Test: `internal/redirectcheck/scope_test.go`

**Interfaces:**
- Consumes: `Rule` (Task 2).
- Produces: `type Scope string` with `ScopeInScope`, `ScopeExternal`, `ScopeDynamic`; `func Classify(rule Rule, domain string) (Scope, error)`; `func resolveURL(raw, domain string) string` (unexported, used by later tasks in the same package); `func normalizeForSitemap(raw string) string` (unexported, used by Tasks 4 and 5 in the same package).

- [ ] **Step 1: Write the failing test**

Create `internal/redirectcheck/scope_test.go`:

```go
package redirectcheck_test

import (
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func rule(original, target string) redirectcheck.Rule {
	return redirectcheck.Rule{Original: original, Target: target}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		r      redirectcheck.Rule
		domain string
		want   redirectcheck.Scope
	}{
		{"relative target on main domain", rule("/old", "/new"), "example.com", redirectcheck.ScopeInScope},
		{"absolute target on main domain", rule("/old", "https://example.com/new"), "example.com", redirectcheck.ScopeInScope},
		{"absolute target on subdomain", rule("/old", "https://shop.example.com/new"), "example.com", redirectcheck.ScopeInScope},
		{"absolute target on external domain", rule("/old", "https://other-site.com/new"), "example.com", redirectcheck.ScopeExternal},
		{"regex original is dynamic", rule("https?://example.com/cases(?P<page_slug>/.*)?$", "https://example.com/en/cases{page_slug}"), "example.com", redirectcheck.ScopeDynamic},
		{"placeholder target is dynamic", rule("/cases", "https://example.com/en/cases{page_slug}"), "example.com", redirectcheck.ScopeDynamic},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := redirectcheck.Classify(c.r, c.domain)
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			if got != c.want {
				t.Errorf("Classify(%+v, %q) = %q, want %q", c.r, c.domain, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/redirectcheck/... -run TestClassify -v`
Expected: FAIL — `undefined: redirectcheck.Classify`

- [ ] **Step 3: Write the implementation**

Create `internal/redirectcheck/scope.go`:

```go
package redirectcheck

import (
	"net/url"
	"strings"
)

// Scope classifies a rule by where its target points, relative to the configured main domain.
type Scope string

const (
	ScopeInScope  Scope = "in-scope"
	ScopeExternal Scope = "external"
	ScopeDynamic  Scope = "dynamic-pattern"
)

// resolveURL turns a rule's Original/Target value into an absolute URL. Relative paths
// ("/foo") resolve against domain over https; absolute values (containing "://") are used
// as given.
func resolveURL(raw, domain string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "://") {
		return raw
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	base := domain
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	return base + raw
}

// isDynamicPattern reports whether a rule uses HubSpot's regex/named-group syntax (Original
// URL) or a {placeholder} substitution (Target) — these can't be resolved to one concrete
// URL without a sample value.
func isDynamicPattern(rule Rule) bool {
	return strings.Contains(rule.Original, "(?P<") ||
		strings.HasPrefix(strings.TrimSpace(rule.Original), "https?://") ||
		strings.Contains(rule.Target, "{")
}

// inScopeHost reports whether host is domain itself or a subdomain of it.
func inScopeHost(host, domain string) bool {
	host = strings.ToLower(host)
	domain = strings.ToLower(domain)
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// Classify determines a rule's Scope. An error is returned only if the target can't be
// parsed as a URL at all.
func Classify(rule Rule, domain string) (Scope, error) {
	if isDynamicPattern(rule) {
		return ScopeDynamic, nil
	}
	u, err := url.Parse(resolveURL(rule.Target, domain))
	if err != nil {
		return "", err
	}
	if inScopeHost(u.Hostname(), domain) {
		return ScopeInScope, nil
	}
	return ScopeExternal, nil
}

// normalizeForSitemap reduces a URL to host+path (dropping scheme, matching sitemap.xml's
// typical canonical-https convention loosely) for membership lookups.
func normalizeForSitemap(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(strings.TrimRight(raw, "/"))
	}
	path := u.Path
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return strings.ToLower(u.Host) + path
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/redirectcheck/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/redirectcheck/scope.go internal/redirectcheck/scope_test.go
git commit -m "Add domain-scope classification for redirect rules"
```

---

### Task 4: Sitemap discovery wrapper

**Files:**
- Create: `internal/redirectcheck/sitemap.go`
- Test: `internal/redirectcheck/sitemap_test.go`

**Interfaces:**
- Consumes: `sitemap.Fetch` (Task 1), `normalizeForSitemap` (Task 3).
- Produces: `func DiscoverSitemap(ctx context.Context, fetcher crawler.Fetcher, domain, override string) (map[string]bool, error)` — returned map keys are `normalizeForSitemap`-normalized (host+path, lowercase, no trailing slash). Also introduces `type fakeFetcher struct{ pages map[string]*crawler.Page }` in the test file — this is reused, unmodified, by Tasks 5 and 6's tests in the same `redirectcheck_test` package (do not redeclare it there).

- [ ] **Step 1: Write the failing test**

Create `internal/redirectcheck/sitemap_test.go`:

```go
package redirectcheck_test

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

// fakeFetcher serves canned responses keyed by URL; unknown URLs return a 404. Shared by every
// test file in this package.
type fakeFetcher struct{ pages map[string]*crawler.Page }

func (f fakeFetcher) Fetch(_ context.Context, rawURL string) (*crawler.Page, error) {
	if p, ok := f.pages[rawURL]; ok {
		return p, nil
	}
	return &crawler.Page{RequestedURL: rawURL, StatusCode: 404}, nil
}

const testSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/a</loc></url>
  <url><loc>https://example.com/b</loc></url>
</urlset>`

func TestDiscoverSitemapDefault(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(testSitemap)},
	}}
	urls, err := redirectcheck.DiscoverSitemap(context.Background(), ff, "example.com", "")
	if err != nil {
		t.Fatalf("DiscoverSitemap: %v", err)
	}
	if !urls["example.com/a"] || !urls["example.com/b"] {
		t.Errorf("got %v, missing expected normalized URLs", urls)
	}
}

func TestDiscoverSitemapOverride(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/custom-sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(testSitemap)},
	}}
	urls, err := redirectcheck.DiscoverSitemap(context.Background(), ff, "example.com", "https://example.com/custom-sitemap.xml")
	if err != nil {
		t.Fatalf("DiscoverSitemap: %v", err)
	}
	if len(urls) != 2 {
		t.Errorf("got %d urls, want 2: %v", len(urls), urls)
	}
}

func TestDiscoverSitemapNotFoundErrors(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{}}
	if _, err := redirectcheck.DiscoverSitemap(context.Background(), ff, "example.com", ""); err == nil {
		t.Fatal("expected an error when no sitemap is reachable")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/redirectcheck/... -run TestDiscoverSitemap -v`
Expected: FAIL — `undefined: redirectcheck.DiscoverSitemap`

- [ ] **Step 3: Write the implementation**

Create `internal/redirectcheck/sitemap.go`:

```go
package redirectcheck

import (
	"context"
	"fmt"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/sitemap"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// DiscoverSitemap fetches and parses the sitemap for domain, returning the normalized set of
// <loc> URLs it declares. If override is non-empty it is used as the only candidate;
// otherwise the conventional /sitemap.xml and /sitemap_index.xml locations are tried. An
// error is returned if nothing usable is found at any candidate — callers should treat this
// as fatal, since sitemap-membership columns would otherwise be silently wrong for every row.
func DiscoverSitemap(ctx context.Context, fetcher crawler.Fetcher, domain, override string) (map[string]bool, error) {
	candidates := map[string]bool{}
	where := domain + "'s default sitemap locations (/sitemap.xml, /sitemap_index.xml)"
	if override != "" {
		candidates[override] = true
		where = override
	} else {
		candidates["https://"+domain+"/sitemap.xml"] = false
		candidates["https://"+domain+"/sitemap_index.xml"] = false
	}

	rawURLs, parsed, _ := sitemap.Fetch(ctx, fetcher, candidates)
	if parsed == 0 {
		return nil, fmt.Errorf("could not fetch/parse a sitemap at %s; pass --sitemap-url to point at the right location", where)
	}

	urls := make(map[string]bool, len(rawURLs))
	for u := range rawURLs {
		urls[normalizeForSitemap(u)] = true
	}
	return urls, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/redirectcheck/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/redirectcheck/sitemap.go internal/redirectcheck/sitemap_test.go
git commit -m "Add redirectcheck.DiscoverSitemap"
```

---

### Task 5: Verdict logic — `CheckRule`

**Files:**
- Create: `internal/redirectcheck/types.go`
- Create: `internal/redirectcheck/check.go`
- Test: `internal/redirectcheck/check_test.go`

**Interfaces:**
- Consumes: `Rule` (Task 2), `resolveURL`/`normalizeForSitemap` (Task 3), `fakeFetcher` (Task 4, same test package).
- Produces: `type Verdict string` with `VerdictOK`, `VerdictWarning`, `VerdictError`, `VerdictSkippedExternal`, `VerdictSkippedDynamic`; `type RowResult struct { Scope Scope; SourceStatus int; SourceFinalURL string; SourceMatchesTarget bool; TargetStatus int; OriginalInSitemap bool; TargetInSitemap bool; Verdict Verdict; Notes []string }`; `func CheckRule(ctx context.Context, fetcher crawler.Fetcher, domain string, rule Rule, sitemapURLs map[string]bool) RowResult`.

- [ ] **Step 1: Write the failing tests**

Create `internal/redirectcheck/check_test.go`:

```go
package redirectcheck_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func baseRule() redirectcheck.Rule {
	return redirectcheck.Rule{
		Original:            "/old",
		Target:              "/new",
		IgnoreTrailingSlash: "TRUE",
		IgnoreProtocol:      "TRUE",
		DisableIfPageExists: "TRUE",
	}
}

func containsSubstring(notes []string, substr string) bool {
	for _, n := range notes {
		if strings.Contains(n, substr) {
			return true
		}
	}
	return false
}

func TestCheckRuleCleanRedirect(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/new", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/new", Status: 301}}},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{"example.com/new": true})
	if res.Verdict != redirectcheck.VerdictOK {
		t.Fatalf("verdict = %q, notes = %v, want ok", res.Verdict, res.Notes)
	}
}

func TestCheckRuleWrongDestination(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/somewhere-else", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/somewhere-else", Status: 301}}},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{"example.com/new": true, "example.com/somewhere-else": true})
	if res.Verdict != redirectcheck.VerdictError {
		t.Fatalf("verdict = %q, notes = %v, want error", res.Verdict, res.Notes)
	}
	if !containsSubstring(res.Notes, "unexpected destination") {
		t.Errorf("notes = %v, want a note about unexpected destination", res.Notes)
	}
}

func TestCheckRule404Target(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/new", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/new", Status: 301}}},
		"https://example.com/new": {StatusCode: 404, FinalURL: "https://example.com/new"},
	}}
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{})
	if res.Verdict != redirectcheck.VerdictError {
		t.Fatalf("verdict = %q, notes = %v, want error", res.Verdict, res.Notes)
	}
	if !containsSubstring(res.Notes, "redirect target returns 404") {
		t.Errorf("notes = %v, want a note about the target returning 404", res.Notes)
	}
}

func TestCheckRuleLiveSourceDisableTrueIsWarning(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/old"},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	rule := baseRule()
	rule.DisableIfPageExists = "TRUE"
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", rule, map[string]bool{"example.com/new": true})
	if res.Verdict != redirectcheck.VerdictWarning {
		t.Fatalf("verdict = %q, notes = %v, want warning", res.Verdict, res.Notes)
	}
}

func TestCheckRuleLiveSourceDisableFalseIsError(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/old"},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	rule := baseRule()
	rule.DisableIfPageExists = "FALSE"
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", rule, map[string]bool{"example.com/new": true})
	if res.Verdict != redirectcheck.VerdictError {
		t.Fatalf("verdict = %q, notes = %v, want error", res.Verdict, res.Notes)
	}
}

func TestCheckRuleStaleSitemapEntry(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/new", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/new", Status: 301}}},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	// The old URL is still listed in the sitemap even though it now redirects away.
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{"example.com/old": true, "example.com/new": true})
	if res.Verdict != redirectcheck.VerdictWarning {
		t.Fatalf("verdict = %q, notes = %v, want warning", res.Verdict, res.Notes)
	}
	if !containsSubstring(res.Notes, "stale sitemap entry") {
		t.Errorf("notes = %v, want a stale-sitemap note", res.Notes)
	}
}

func TestCheckRuleTargetNotInSitemap(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {StatusCode: 200, FinalURL: "https://example.com/new", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/new", Status: 301}}},
		"https://example.com/new": {StatusCode: 200, FinalURL: "https://example.com/new"},
	}}
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{})
	if res.Verdict != redirectcheck.VerdictWarning {
		t.Fatalf("verdict = %q, notes = %v, want warning", res.Verdict, res.Notes)
	}
	if !containsSubstring(res.Notes, "target not confirmed in sitemap") {
		t.Errorf("notes = %v, want a target-not-in-sitemap note", res.Notes)
	}
}

func TestCheckRuleSourceFetchError(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/old": {RequestedURL: "https://example.com/old", Err: "connection reset"},
	}}
	res := redirectcheck.CheckRule(context.Background(), ff, "example.com", baseRule(), map[string]bool{})
	if res.Verdict != redirectcheck.VerdictError {
		t.Fatalf("verdict = %q, want error", res.Verdict)
	}
	if !containsSubstring(res.Notes, "connection reset") {
		t.Errorf("notes = %v, want the fetch error message", res.Notes)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/redirectcheck/... -run TestCheckRule -v`
Expected: FAIL — `undefined: redirectcheck.CheckRule` (and related types)

- [ ] **Step 3: Write the implementation**

Create `internal/redirectcheck/types.go`:

```go
package redirectcheck

// Verdict is the overall pass/fail assessment of one row.
type Verdict string

const (
	VerdictOK              Verdict = "ok"
	VerdictWarning         Verdict = "warning"
	VerdictError           Verdict = "error"
	VerdictSkippedExternal Verdict = "skipped-external"
	VerdictSkippedDynamic  Verdict = "skipped-dynamic"
)

// RowResult is what checking one rule against the live site produced.
type RowResult struct {
	Scope               Scope
	SourceStatus        int
	SourceFinalURL      string
	SourceMatchesTarget bool
	TargetStatus        int
	OriginalInSitemap   bool
	TargetInSitemap     bool
	Verdict             Verdict
	Notes               []string
}

var verdictRank = map[Verdict]int{
	"":             0,
	VerdictOK:      0,
	VerdictWarning: 1,
	VerdictError:   2,
}

// escalate returns whichever of current/next is the more severe verdict, so a row's Verdict
// always reflects the worst finding triggered while its Notes list every finding.
func escalate(current, next Verdict) Verdict {
	if verdictRank[next] > verdictRank[current] {
		return next
	}
	return current
}
```

Create `internal/redirectcheck/check.go`:

```go
package redirectcheck

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// CheckRule fetches rule's Original and Target URLs against the live site and returns the
// resulting verdict. sitemapURLs is the set discovered by DiscoverSitemap. Callers should only
// call this for rules classified ScopeInScope.
func CheckRule(ctx context.Context, fetcher crawler.Fetcher, domain string, rule Rule, sitemapURLs map[string]bool) RowResult {
	res := RowResult{Scope: ScopeInScope}
	originalURL := resolveURL(rule.Original, domain)
	targetURL := resolveURL(rule.Target, domain)

	sourcePage, sourceErr := fetcher.Fetch(ctx, originalURL)
	if sourceErr != nil || sourcePage == nil || sourcePage.Err != "" {
		res.Verdict = VerdictError
		res.Notes = append(res.Notes, "source fetch failed: "+fetchErrString(sourcePage, sourceErr))
		return res
	}

	res.SourceStatus = sourcePage.StatusCode
	res.SourceFinalURL = sourcePage.FinalURL

	ignoreProtocol := strings.EqualFold(rule.IgnoreProtocol, "TRUE")
	ignoreTrailingSlash := strings.EqualFold(rule.IgnoreTrailingSlash, "TRUE")
	disableIfExists := strings.EqualFold(rule.DisableIfPageExists, "TRUE")

	res.SourceMatchesTarget = urlsMatch(sourcePage.FinalURL, targetURL, ignoreProtocol, ignoreTrailingSlash)

	switch {
	case sourcePage.StatusCode >= 400:
		res.Verdict = escalate(res.Verdict, VerdictError)
		res.Notes = append(res.Notes, fmt.Sprintf("source URL returns %d", sourcePage.StatusCode))
	case len(sourcePage.Redirects) == 0:
		if disableIfExists {
			res.Verdict = escalate(res.Verdict, VerdictWarning)
			res.Notes = append(res.Notes, "redirect suppressed by config — source page still exists, consider removing rule")
		} else {
			res.Verdict = escalate(res.Verdict, VerdictError)
			res.Notes = append(res.Notes, "rule requires unconditional redirect but source is still live")
		}
	case !res.SourceMatchesTarget:
		res.Verdict = escalate(res.Verdict, VerdictError)
		res.Notes = append(res.Notes, fmt.Sprintf("redirects to unexpected destination %q", sourcePage.FinalURL))
	}

	targetPage, targetErr := fetcher.Fetch(ctx, targetURL)
	switch {
	case targetErr != nil || targetPage == nil || targetPage.Err != "":
		res.Verdict = escalate(res.Verdict, VerdictError)
		res.Notes = append(res.Notes, "target fetch failed: "+fetchErrString(targetPage, targetErr))
	default:
		res.TargetStatus = targetPage.StatusCode
		if targetPage.StatusCode >= 400 {
			res.Verdict = escalate(res.Verdict, VerdictError)
			res.Notes = append(res.Notes, fmt.Sprintf("redirect target returns %d", targetPage.StatusCode))
		}
	}

	res.OriginalInSitemap = sitemapURLs[normalizeForSitemap(originalURL)]
	res.TargetInSitemap = sitemapURLs[normalizeForSitemap(targetURL)]

	if res.OriginalInSitemap {
		res.Verdict = escalate(res.Verdict, VerdictWarning)
		res.Notes = append(res.Notes, "stale sitemap entry — source URL still listed as canonical")
	}
	if !res.TargetInSitemap {
		res.Verdict = escalate(res.Verdict, VerdictWarning)
		res.Notes = append(res.Notes, "target not confirmed in sitemap")
	}

	if res.Verdict == "" {
		res.Verdict = VerdictOK
	}
	return res
}

func fetchErrString(page *crawler.Page, err error) string {
	if page != nil && page.Err != "" {
		return page.Err
	}
	if err != nil {
		return err.Error()
	}
	return "unknown error"
}

// urlsMatch compares two absolute URLs for equality, treating scheme/trailing-slash
// differences as insignificant when the corresponding rule flags request it.
func urlsMatch(a, b string, ignoreProtocol, ignoreTrailingSlash bool) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	if errA != nil || errB != nil {
		return a == b
	}
	hostA, hostB := strings.ToLower(ua.Host), strings.ToLower(ub.Host)
	pathA, pathB := ua.Path, ub.Path
	if ignoreTrailingSlash {
		pathA = strings.TrimRight(pathA, "/")
		pathB = strings.TrimRight(pathB, "/")
	}
	if hostA != hostB || pathA != pathB {
		return false
	}
	if !ignoreProtocol && !strings.EqualFold(ua.Scheme, ub.Scheme) {
		return false
	}
	return true
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/redirectcheck/... -v`
Expected: PASS — every test in the package, including Tasks 2-4's.

- [ ] **Step 5: Commit**

```bash
git add internal/redirectcheck/types.go internal/redirectcheck/check.go internal/redirectcheck/check_test.go
git commit -m "Add redirectcheck.CheckRule verdict logic"
```

---

### Task 6: Concurrent orchestration — `Run`

**Files:**
- Create: `internal/redirectcheck/run.go`
- Test: `internal/redirectcheck/run_test.go`

**Interfaces:**
- Consumes: `Classify` (Task 3), `DiscoverSitemap` (Task 4), `CheckRule` (Task 5), `fakeFetcher`/`testSitemap` (Task 4, same test package).
- Produces: `type RunOptions struct { Domain, SitemapURL string; Fetcher crawler.Fetcher; Concurrency int; RatePerSecond float64 }`; `func Run(ctx context.Context, rules []Rule, opts RunOptions) ([]RowResult, error)`.

- [ ] **Step 1: Write the failing tests**

Create `internal/redirectcheck/run_test.go`:

```go
package redirectcheck_test

import (
	"context"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func TestRunOrdersResultsAndSkipsOutOfScope(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{
		"https://example.com/sitemap.xml": {StatusCode: 200, ContentType: "application/xml", Body: []byte(testSitemap)},
		"https://example.com/old":         {StatusCode: 200, FinalURL: "https://example.com/a", Redirects: []crawler.Redirect{{From: "https://example.com/old", To: "https://example.com/a", Status: 301}}},
		"https://example.com/a":           {StatusCode: 200, FinalURL: "https://example.com/a"},
	}}
	rules := []redirectcheck.Rule{
		{Original: "/old", Target: "/a", IgnoreTrailingSlash: "TRUE", IgnoreProtocol: "TRUE", DisableIfPageExists: "TRUE"},
		{Original: "/gone", Target: "https://other-site.com/somewhere", DisableIfPageExists: "TRUE"},
	}
	results, err := redirectcheck.Run(context.Background(), rules, redirectcheck.RunOptions{
		Domain: "example.com", Fetcher: ff, Concurrency: 2,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Verdict != redirectcheck.VerdictOK {
		t.Errorf("row 0 verdict = %q, notes = %v, want ok", results[0].Verdict, results[0].Notes)
	}
	if results[1].Verdict != redirectcheck.VerdictSkippedExternal {
		t.Errorf("row 1 verdict = %q, want skipped-external", results[1].Verdict)
	}
}

func TestRunErrorsWhenSitemapUnreachable(t *testing.T) {
	ff := fakeFetcher{pages: map[string]*crawler.Page{}}
	rules := []redirectcheck.Rule{{Original: "/old", Target: "/new"}}
	if _, err := redirectcheck.Run(context.Background(), rules, redirectcheck.RunOptions{Domain: "example.com", Fetcher: ff}); err == nil {
		t.Fatal("expected an error when the sitemap can't be reached")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/redirectcheck/... -run TestRun -v`
Expected: FAIL — `undefined: redirectcheck.Run`

- [ ] **Step 3: Write the implementation**

Create `internal/redirectcheck/run.go`:

```go
package redirectcheck

import (
	"context"
	"sync"

	"golang.org/x/time/rate"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// RunOptions configures a check-redirects run.
type RunOptions struct {
	Domain        string
	SitemapURL    string
	Fetcher       crawler.Fetcher
	Concurrency   int
	RatePerSecond float64
}

// Run classifies every rule, fetches the site's sitemap once, then checks each in-scope rule
// concurrently (bounded by opts.Concurrency and opts.RatePerSecond). Results are returned in
// the same order as rules. Rows classified external or dynamic-pattern are not fetched.
func Run(ctx context.Context, rules []Rule, opts RunOptions) ([]RowResult, error) {
	sitemapURLs, err := DiscoverSitemap(ctx, opts.Fetcher, opts.Domain, opts.SitemapURL)
	if err != nil {
		return nil, err
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	var limiter *rate.Limiter
	if opts.RatePerSecond > 0 {
		limiter = rate.NewLimiter(rate.Limit(opts.RatePerSecond), 1)
	}

	results := make([]RowResult, len(rules))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, rule := range rules {
		scope, err := Classify(rule, opts.Domain)
		if err != nil {
			results[i] = RowResult{Verdict: VerdictError, Notes: []string{"could not classify rule: " + err.Error()}}
			continue
		}
		if scope == ScopeExternal {
			results[i] = RowResult{Scope: ScopeExternal, Verdict: VerdictSkippedExternal}
			continue
		}
		if scope == ScopeDynamic {
			results[i] = RowResult{Scope: ScopeDynamic, Verdict: VerdictSkippedDynamic}
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(i int, rule Rule) {
			defer wg.Done()
			defer func() { <-sem }()
			if limiter != nil {
				_ = limiter.Wait(ctx)
			}
			results[i] = CheckRule(ctx, opts.Fetcher, opts.Domain, rule, sitemapURLs)
		}(i, rule)
	}
	wg.Wait()
	return results, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/redirectcheck/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/redirectcheck/run.go internal/redirectcheck/run_test.go
git commit -m "Add redirectcheck.Run for concurrent rule checking"
```

---

### Task 7: CSV report writer

**Files:**
- Create: `internal/redirectcheck/report.go`
- Test: `internal/redirectcheck/report_test.go`

**Interfaces:**
- Consumes: `Rule` (Task 2), `RowResult` (Task 5).
- Produces: `func WriteCSV(w io.Writer, rules []Rule, results []RowResult) error`.

- [ ] **Step 1: Write the failing test**

Create `internal/redirectcheck/report_test.go`:

```go
package redirectcheck_test

import (
	"bytes"
	"encoding/csv"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func TestWriteCSVIncludesAllRowsInOrder(t *testing.T) {
	rules := []redirectcheck.Rule{
		{Original: "/old", Target: "/new"},
		{Original: "/gone", Target: "https://other-site.com/x"},
	}
	results := []redirectcheck.RowResult{
		{Scope: redirectcheck.ScopeInScope, SourceStatus: 301, SourceFinalURL: "https://example.com/new", SourceMatchesTarget: true, TargetStatus: 200, TargetInSitemap: true, Verdict: redirectcheck.VerdictOK},
		{Scope: redirectcheck.ScopeExternal, Verdict: redirectcheck.VerdictSkippedExternal},
	}
	var buf bytes.Buffer
	if err := redirectcheck.WriteCSV(&buf, rules, results); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("reading back CSV: %v", err)
	}
	if len(rows) != 3 { // header + 2 data rows
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	if rows[0][len(rows[0])-1] != "notes" {
		t.Errorf("last header column = %q, want notes", rows[0][len(rows[0])-1])
	}
	if rows[1][len(rows[1])-2] != "ok" {
		t.Errorf("row 1 verdict = %q, want ok", rows[1][len(rows[1])-2])
	}
	if rows[2][len(rows[2])-2] != "skipped-external" {
		t.Errorf("row 2 verdict = %q, want skipped-external", rows[2][len(rows[2])-2])
	}
	if rows[2][10] != "external" { // first appended column ("scope")
		t.Errorf("row 2 scope = %q, want external", rows[2][10])
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/redirectcheck/... -run TestWriteCSV -v`
Expected: FAIL — `undefined: redirectcheck.WriteCSV`

- [ ] **Step 3: Write the implementation**

Create `internal/redirectcheck/report.go`:

```go
package redirectcheck

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

// WriteCSV writes rules and their check results as a CSV: the original columns verbatim, plus
// the appended verdict columns. Every input row is present in the output, in order.
func WriteCSV(w io.Writer, rules []Rule, results []RowResult) error {
	cw := csv.NewWriter(w)
	header := append(append([]string{}, expectedHeader...),
		"scope", "source_status", "source_final_url", "source_matches_target",
		"target_status", "original_in_sitemap", "target_in_sitemap", "verdict", "notes",
	)
	if err := cw.Write(header); err != nil {
		return err
	}
	for i, rule := range rules {
		res := results[i]
		record := []string{
			rule.Original, rule.Target, rule.RedirectType, rule.RedirectStyle, rule.Priority,
			rule.MatchQueryStrings, rule.IgnoreTrailingSlash, rule.IgnoreProtocol, rule.DisableIfPageExists, rule.Note,
			string(res.Scope),
			statusStr(res.SourceStatus), res.SourceFinalURL, boolStr(res.SourceMatchesTarget),
			statusStr(res.TargetStatus), boolStr(res.OriginalInSitemap), boolStr(res.TargetInSitemap),
			string(res.Verdict), strings.Join(res.Notes, "; "),
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func statusStr(code int) string {
	if code == 0 {
		return ""
	}
	return strconv.Itoa(code)
}

func boolStr(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/redirectcheck/... -v`
Expected: PASS — the whole `internal/redirectcheck` package.

- [ ] **Step 5: Commit**

```bash
git add internal/redirectcheck/report.go internal/redirectcheck/report_test.go
git commit -m "Add redirectcheck.WriteCSV report writer"
```

---

### Task 8: CLI subcommand `gocrawl check-redirects`

**Files:**
- Create: `cmd/gocrawl/checkredirects.go`
- Modify: `cmd/gocrawl/main.go`
- Test: `cmd/gocrawl/checkredirects_test.go`

**Interfaces:**
- Consumes: `redirectcheck.ParseCSV`, `redirectcheck.Run`, `redirectcheck.RunOptions`, `redirectcheck.WriteCSV` (Tasks 2, 6, 7); `crawler.NewHTTPFetcher`, `crawler.Options` (existing).
- Produces: `func newCheckRedirectsCmd() *cobra.Command`, registered in `newRootCmd()`.

- [ ] **Step 1: Write the failing tests**

Create `cmd/gocrawl/checkredirects_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRedirectsRequiresInputAndDomain(t *testing.T) {
	cmd := newCheckRedirectsCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error when --input and --domain are missing")
	}
}

func TestCheckRedirectsErrorsOnMissingInputFile(t *testing.T) {
	cmd := newCheckRedirectsCmd()
	cmd.SetArgs([]string{"--input", filepath.Join(t.TempDir(), "does-not-exist.csv"), "--domain", "example.com"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for a nonexistent input file")
	}
	if !strings.Contains(err.Error(), "opening input") {
		t.Errorf("error = %v, want it to mention opening the input file", err)
	}
}

func TestCheckRedirectsErrorsOnBadCSVSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(path, []byte("\"Original URL\",\"Redirect to\"\n\"/a\",\"/b\"\n"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	cmd := newCheckRedirectsCmd()
	cmd.SetArgs([]string{"--input", path, "--domain", "example.com"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for a CSV with the wrong column schema")
	}
	if !strings.Contains(err.Error(), "parsing input CSV") {
		t.Errorf("error = %v, want it to mention parsing the input CSV", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./cmd/gocrawl/... -run TestCheckRedirects -v`
Expected: FAIL — `undefined: newCheckRedirectsCmd`

- [ ] **Step 3: Write the implementation**

Create `cmd/gocrawl/checkredirects.go`:

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func newCheckRedirectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-redirects",
		Short: "Verify a redirect-rule CSV export against a live site",
		Long: "check-redirects reads a HubSpot-format URL Redirects CSV export and checks each\n" +
			"in-scope rule against the live site: whether the source still redirects correctly,\n" +
			"whether the target is a live page, and whether both agree with the current sitemap.xml.\n" +
			"It writes the input CSV back out with appended verdict columns.",
		RunE: runCheckRedirects,
	}
	f := cmd.Flags()
	f.String("input", "", "path to the redirect-rule CSV (required)")
	f.String("domain", "", "main domain; subdomains are in-scope, other domains are skipped (required)")
	f.String("output", "", "output CSV path (default: stdout)")
	f.String("sitemap-url", "", "sitemap URL to use if the default locations (/sitemap.xml, /sitemap_index.xml) don't work")
	f.Int("concurrency", 4, "parallel fetch workers")
	f.Float64("rate", 0, "max requests per second (0 = unlimited)")
	f.Duration("timeout", 15*time.Second, "per-request timeout")
	f.String("user-agent", "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)", "User-Agent header")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("domain")
	return cmd
}

func runCheckRedirects(cmd *cobra.Command, _ []string) error {
	f := cmd.Flags()
	inputPath, _ := f.GetString("input")
	domain, _ := f.GetString("domain")
	outputPath, _ := f.GetString("output")
	sitemapURL, _ := f.GetString("sitemap-url")
	concurrency, _ := f.GetInt("concurrency")
	rateLimit, _ := f.GetFloat64("rate")
	timeout, _ := f.GetDuration("timeout")
	userAgent, _ := f.GetString("user-agent")

	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("opening input: %w", err)
	}
	defer file.Close()

	rules, err := redirectcheck.ParseCSV(file)
	if err != nil {
		return fmt.Errorf("parsing input CSV: %w", err)
	}

	fetcher := crawler.NewHTTPFetcher(crawler.Options{
		Timeout:      timeout,
		UserAgent:    userAgent,
		MaxRedirects: 10,
		MaxBodyBytes: 5 << 20,
	})

	results, err := redirectcheck.Run(cmd.Context(), rules, redirectcheck.RunOptions{
		Domain:        domain,
		SitemapURL:    sitemapURL,
		Fetcher:       fetcher,
		Concurrency:   concurrency,
		RatePerSecond: rateLimit,
	})
	if err != nil {
		return err
	}

	out := os.Stdout
	if outputPath != "" {
		outFile, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("creating output: %w", err)
		}
		defer outFile.Close()
		out = outFile
	}
	if err := redirectcheck.WriteCSV(out, rules, results); err != nil {
		return fmt.Errorf("writing output CSV: %w", err)
	}
	if outputPath != "" {
		fmt.Fprintf(os.Stderr, "Report written to %s\n", outputPath)
	}
	return nil
}
```

Modify `cmd/gocrawl/main.go`: add `root.AddCommand(newCheckRedirectsCmd())` to `newRootCmd()`, alongside the existing `AddCommand` calls (after `root.AddCommand(newPathCmd())`):

```go
	root.AddCommand(newPathCmd())
	root.AddCommand(newCheckRedirectsCmd())
	return root
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./cmd/gocrawl/... -run TestCheckRedirects -v`
Expected: PASS

- [ ] **Step 5: Run the full test suite and vet**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all PASS, no vet warnings.

- [ ] **Step 6: Commit**

```bash
git add cmd/gocrawl/checkredirects.go cmd/gocrawl/checkredirects_test.go cmd/gocrawl/main.go
git commit -m "Add gocrawl check-redirects CLI subcommand"
```

---

### Task 9: Documentation

**Files:**
- Create: `docs/redirect-check.md`
- Modify: `docs/README.md`
- Modify: `README.md`

**Interfaces:** None (documentation only).

- [ ] **Step 1: Write `docs/redirect-check.md`**

```markdown
# Verifying a redirect-rule export (`check-redirects`)

`gocrawl check-redirects` verifies a redirect-rule CSV export — the format produced by
HubSpot's URL Redirects tool — against a live site. It answers: are these redirects actually
working, and do they point at live pages rather than 404s?

## Usage

```sh
gocrawl check-redirects --input redirects.csv --domain example.com --output results.csv
```

| Flag | Meaning |
| --- | --- |
| `--input` (required) | Path to the redirect-rule CSV. |
| `--domain` (required) | Main domain. Subdomains are automatically in-scope; every other domain is skipped as external. |
| `--output` | Output CSV path (default: stdout). |
| `--sitemap-url` | Sitemap URL to use if `/sitemap.xml` and `/sitemap_index.xml` aren't reachable. |
| `--concurrency` | Parallel fetch workers (default 4). |
| `--rate` | Max requests per second (default unlimited). |
| `--timeout` | Per-request timeout (default 15s). |
| `--user-agent` | User-Agent header to send. |

## Input schema

The CSV must have exactly these columns, in order (HubSpot's export format):

```
"Original URL","Redirect to","Redirect type","Redirect style","Priority",
"Match query strings","Ignore trailing slash","Ignore protocol","Disable if page exists","Note"
```

Rows are classified before fetching:

- **in-scope** — the target resolves to `--domain` or a subdomain of it. Checked.
- **external** — the target resolves to a different domain. Skipped (not fetched), to avoid
  an uncontrolled cross-domain crawl.
- **dynamic-pattern** — the rule uses HubSpot's regex/named-group syntax or a `{placeholder}`
  substitution in the target, so it can't be resolved to one concrete URL. Skipped.

## Output

The input CSV is echoed back with these columns appended:

| Column | Meaning |
| --- | --- |
| `scope` | `in-scope` / `external` / `dynamic-pattern` |
| `source_status` | HTTP status of the Original URL, after following any redirects |
| `source_final_url` | Where the Original URL actually ended up |
| `source_matches_target` | Does the source's final URL match `Redirect to`? |
| `target_status` | HTTP status of `Redirect to`, after following any further redirects |
| `original_in_sitemap` | Is the Original URL still listed in the sitemap? |
| `target_in_sitemap` | Is the target listed in the sitemap? |
| `verdict` | `ok` / `warning` / `error` / `skipped-external` / `skipped-dynamic` |
| `notes` | Every finding that contributed to the verdict, semicolon-separated |

A row is flagged `error` when: the source can't be fetched, the source no longer redirects
and `Disable if page exists` is `FALSE`, the source redirects somewhere other than the
expected target, or the target itself returns a 4xx/5xx. A row is flagged `warning` when the
source no longer redirects but `Disable if page exists` is `TRUE` (HubSpot's own suppression —
worth a look, not necessarily broken), the source is still listed in the sitemap as canonical
(stale entry), or the target isn't found in the sitemap.
```

- [ ] **Step 2: Link it from `docs/README.md`**

In `docs/README.md`, add a row to the table (after the "Storage & comparison" row):

```markdown
| [Redirect-rule verification](redirect-check.md) | Checking a HubSpot-style redirect-rule CSV export against a live site with `gocrawl check-redirects`. |
```

- [ ] **Step 3: Link it from the root `README.md`**

In `README.md`, add one example to the "Quick start" code block (after the `gocrawl init` example, before the closing ```` ``` ````):

```markdown
# Verify a HubSpot redirect-rule export against the live site
gocrawl check-redirects --input redirects.csv --domain example.com --output results.csv
```

And add a line to the "Documentation" list:

```markdown
- [Redirect-rule verification](docs/redirect-check.md) — checking a redirect-rule CSV export against a live site.
```

- [ ] **Step 4: Verify the docs build/render sanely**

Run: `grep -c "check-redirects" README.md docs/README.md docs/redirect-check.md`
Expected: each file returns a non-zero count.

- [ ] **Step 5: Commit**

```bash
git add docs/redirect-check.md docs/README.md README.md
git commit -m "Document the check-redirects CLI subcommand"
```

---

## Final verification

After Task 9, run the full project gate once more:

```bash
gofmt -l .
go vet ./...
go build ./...
go test ./...
```

Expected: `gofmt -l .` prints nothing (no unformatted files), and everything else passes clean.
