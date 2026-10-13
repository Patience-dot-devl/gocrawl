package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

// redirectTestOrigin is the domain the tests check. hostRewriteFetcher sends its requests to
// an httptest site, because redirectcheck always builds https://<domain>/... URLs and an
// httptest URL has a port that the domain scope rules would reject.
const redirectTestOrigin = "https://example.com"

const redirectCSVHeader = `"Original URL","Redirect to","Redirect type","Redirect style","Priority",` +
	`"Match query strings","Ignore trailing slash","Ignore protocol","Disable if page exists","Note"` + "\n"

const redirectCSV = redirectCSVHeader +
	"/old,/a,301,STANDARD,1,FALSE,TRUE,TRUE,FALSE,moved\n" +
	"/gone,https://other.example/x,301,STANDARD,2,FALSE,TRUE,TRUE,FALSE,\n"

const redirectTestSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/a</loc></url>
</urlset>`

// redirectTestSite serves a small site: a sitemap listing /a, a 301 from /old to /a, and /a.
// A non-nil gate holds every request until the test ends or the request is canceled, which
// keeps a check "running" for the cap, cancel and export tests. Tests must not close the
// gate themselves.
func redirectTestSite(t *testing.T, gate chan struct{}) *httptest.Server {
	t.Helper()
	wait := func(r *http.Request) {
		if gate == nil {
			return
		}
		select {
		case <-gate:
		case <-r.Context().Done():
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		wait(r)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, redirectTestSitemap)
	})
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		wait(r)
		http.Redirect(w, r, "/a", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		wait(r)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><head><title>A</title></head><body>a</body></html>")
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	if gate != nil {
		// Cleanups run last-in first-out: release blocked handlers before ts.Close waits on them.
		t.Cleanup(func() { close(gate) })
	}
	return ts
}

// hostRewriteFetcher sends requests for redirectTestOrigin to the httptest site through the
// real redirectcheck fetcher and rewrites the URLs on the way back.
type hostRewriteFetcher struct {
	base  string
	inner crawler.Fetcher
}

func (f hostRewriteFetcher) Fetch(ctx context.Context, rawURL string) (*crawler.Page, error) {
	page, err := f.inner.Fetch(ctx, strings.Replace(rawURL, redirectTestOrigin, f.base, 1))
	if page != nil {
		back := func(s string) string { return strings.Replace(s, f.base, redirectTestOrigin, 1) }
		page.RequestedURL = back(page.RequestedURL)
		page.FinalURL = back(page.FinalURL)
		for i := range page.Redirects {
			page.Redirects[i].From = back(page.Redirects[i].From)
			page.Redirects[i].To = back(page.Redirects[i].To)
		}
	}
	return page, err
}

// useRedirectTestSite points new redirect-check jobs at site until the test ends. The handler
// reads newRedirectFetcher synchronously, so swapping it here never races a job goroutine.
func useRedirectTestSite(t *testing.T, site *httptest.Server) {
	t.Helper()
	prev := newRedirectFetcher
	newRedirectFetcher = func() crawler.Fetcher {
		return hostRewriteFetcher{base: site.URL, inner: redirectcheck.NewFetcher(5*time.Second, "gocrawl-test")}
	}
	t.Cleanup(func() { newRedirectFetcher = prev })
}

// postRedirectCheckForm sends POST /api/redirect-checks as multipart through the full handler.
// An empty csvName sends no file part.
func postRedirectCheckForm(t *testing.T, srv *Server, fields map[string]string, csvName, csv, origin string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if csvName != "" {
		fw, err := mw.CreateFormFile("file", csvName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(csv)); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/redirect-checks", &buf)
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var out map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode response %q: %v", rec.Body.String(), err)
		}
	}
	return rec, out
}

// pollRedirectCheck polls GET /api/redirect-checks/{id} until the status is not "running".
func pollRedirectCheck(t *testing.T, srv *Server, id string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		rec, out := doJSON(t, srv, http.MethodGet, "/api/redirect-checks/"+id, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("poll: status = %d, body = %s", rec.Code, rec.Body)
		}
		if out["status"] != "running" {
			return out
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("redirect check %s did not finish within the test deadline", id)
	return nil
}

func startRedirectCheckForTest(t *testing.T, srv *Server, fields map[string]string) string {
	t.Helper()
	rec, out := postRedirectCheckForm(t, srv, fields, "redirects.csv", redirectCSV, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start: status = %d, body = %s", rec.Code, rec.Body)
	}
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("start: no id in %v", out)
	}
	return id
}

func TestRedirectCheckRunsToCompletion(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)

	rec, started := postRedirectCheckForm(t, srv, map[string]string{
		"domain": "https://Example.com/", "concurrency": "2", "rate": "0", "adaptive_delay": "false",
	}, "redirects.csv", redirectCSV, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if started["status"] != "running" || started["total"] != float64(2) || started["done"] != float64(0) {
		t.Fatalf("start view = %v", started)
	}
	if started["domain"] != "example.com" || started["file_name"] != "redirects.csv" {
		t.Fatalf("start view domain/file = %v / %v", started["domain"], started["file_name"])
	}
	if _, ok := started["rows"]; ok {
		t.Fatalf("a running check must not carry rows: %v", started)
	}

	job := pollRedirectCheck(t, srv, started["id"].(string))
	if job["status"] != "done" {
		t.Fatalf("status = %v, error = %v", job["status"], job["error"])
	}
	if job["done"] != float64(2) || job["total"] != float64(2) {
		t.Fatalf("done/total = %v/%v, want 2/2", job["done"], job["total"])
	}
	rows, _ := job["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("rows = %v", job["rows"])
	}
	first := rows[0].(map[string]any)
	if first["original"] != "/old" || first["note"] != "moved" || first["verdict"] != "ok" ||
		first["source_final_url"] != "https://example.com/a" || first["target_status"] != float64(200) ||
		first["target_in_sitemap"] != true || first["scope"] != "in-scope" {
		t.Errorf("row 0 = %v", first)
	}
	second := rows[1].(map[string]any)
	if second["verdict"] != "skipped-external" || second["scope"] != "external" {
		t.Errorf("row 1 = %v", second)
	}
}

func TestListRedirectChecks(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)

	rec, out := doJSON(t, srv, http.MethodGet, "/api/redirect-checks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("empty list: status = %d", rec.Code)
	}
	if checks, ok := out["checks"].([]any); !ok || len(checks) != 0 {
		t.Fatalf("empty list = %v, want checks: []", out)
	}

	id := startRedirectCheckForTest(t, srv, map[string]string{"domain": "example.com"})
	pollRedirectCheck(t, srv, id)
	_, out = doJSON(t, srv, http.MethodGet, "/api/redirect-checks", nil)
	checks, _ := out["checks"].([]any)
	if len(checks) != 1 {
		t.Fatalf("checks = %v", out["checks"])
	}
	c := checks[0].(map[string]any)
	if c["id"] != id || c["status"] != "done" || c["domain"] != "example.com" {
		t.Fatalf("list item = %v", c)
	}
	if _, ok := c["rows"]; ok {
		t.Fatal("list items must not carry rows")
	}
}

func TestRedirectCheckWithoutSitemapEndsInError(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)
	id := startRedirectCheckForTest(t, srv, map[string]string{
		"domain": "example.com", "sitemap_url": "https://example.com/missing.xml",
	})
	job := pollRedirectCheck(t, srv, id)
	if job["status"] != "error" {
		t.Fatalf("status = %v, want error", job["status"])
	}
	if msg, _ := job["error"].(string); !strings.Contains(msg, "sitemap") {
		t.Fatalf("error = %q, want it to explain the sitemap was not found", msg)
	}
	if _, ok := job["rows"]; ok {
		t.Fatal("a failed check must not carry rows")
	}
}

func TestStartRedirectCheckRejectsBadInput(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)
	badHeader := "\"Original URL\",\"Redirect to\"\n/a,/b\n"
	cases := []struct {
		name    string
		fields  map[string]string
		csvName string
		csv     string
	}{
		{"missing domain", map[string]string{}, "r.csv", redirectCSV},
		{"domain with a path", map[string]string{"domain": "example.com/blog"}, "r.csv", redirectCSV},
		{"concurrency zero", map[string]string{"domain": "example.com", "concurrency": "0"}, "r.csv", redirectCSV},
		{"concurrency too high", map[string]string{"domain": "example.com", "concurrency": "99"}, "r.csv", redirectCSV},
		{"concurrency not a number", map[string]string{"domain": "example.com", "concurrency": "abc"}, "r.csv", redirectCSV},
		{"negative rate", map[string]string{"domain": "example.com", "rate": "-1"}, "r.csv", redirectCSV},
		{"rate NaN", map[string]string{"domain": "example.com", "rate": "NaN"}, "r.csv", redirectCSV},
		{"rate not a number", map[string]string{"domain": "example.com", "rate": "fast"}, "r.csv", redirectCSV},
		{"adaptive delay not a bool", map[string]string{"domain": "example.com", "adaptive_delay": "maybe"}, "r.csv", redirectCSV},
		{"sitemap url not http", map[string]string{"domain": "example.com", "sitemap_url": "ftp://example.com/s.xml"}, "r.csv", redirectCSV},
		{"missing file", map[string]string{"domain": "example.com"}, "", ""},
		{"wrong CSV header", map[string]string{"domain": "example.com"}, "r.csv", badHeader},
		{"header but no rules", map[string]string{"domain": "example.com"}, "r.csv", redirectCSVHeader},
	}
	for _, tc := range cases {
		rec, out := postRedirectCheckForm(t, srv, tc.fields, tc.csvName, tc.csv, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (body %s)", tc.name, rec.Code, rec.Body)
			continue
		}
		if msg, _ := out["error"].(string); msg == "" {
			t.Errorf("%s: no error message", tc.name)
		}
	}
	if n := len(srv.redirects.list()); n != 0 {
		t.Fatalf("%d job(s) started from invalid requests", n)
	}
}

func TestNormalizeRedirectDomain(t *testing.T) {
	for in, want := range map[string]string{
		"example.com":                   "example.com",
		"  Example.COM ":                "example.com",
		"example.com/":                  "example.com",
		"https://example.com/":          "example.com",
		"http://shop.example.com/a?b=c": "shop.example.com",
	} {
		got, err := normalizeRedirectDomain(in)
		if err != nil || got != want {
			t.Errorf("normalizeRedirectDomain(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "   ", "example.com/blog", "https://", "exa mple.com"} {
		if got, err := normalizeRedirectDomain(in); err == nil {
			t.Errorf("normalizeRedirectDomain(%q) = %q, want an error", in, got)
		}
	}
}

func TestStartRedirectCheckGuards(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, nil))
	srv := newTestServer(t)

	rec, _ := postRedirectCheckForm(t, srv, map[string]string{"domain": "example.com"}, "r.csv", redirectCSV, "http://evil.example")
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin: status = %d, want 403", rec.Code)
	}
	rec, _ = doJSON(t, srv, http.MethodPost, "/api/redirect-checks", map[string]any{"domain": "example.com"})
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("JSON body: status = %d, want 415", rec.Code)
	}

	prev := maxRedirectCSVBytes
	maxRedirectCSVBytes = 1024
	t.Cleanup(func() { maxRedirectCSVBytes = prev })
	big := redirectCSV + strings.Repeat("/x,/a,301,STANDARD,1,FALSE,TRUE,TRUE,FALSE,\n", 100)
	rec, _ = postRedirectCheckForm(t, srv, map[string]string{"domain": "example.com"}, "r.csv", big, "")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized: status = %d, want 413", rec.Code)
	}
	if n := len(srv.redirects.list()); n != 0 {
		t.Fatalf("%d job(s) started despite the guards", n)
	}
}

func TestRedirectCheckCapReturns429(t *testing.T) {
	useRedirectTestSite(t, redirectTestSite(t, make(chan struct{})))
	srv := newTestServer(t)
	for i := 0; i < maxRunningRedirectChecks; i++ {
		startRedirectCheckForTest(t, srv, map[string]string{"domain": "example.com"})
	}
	rec, out := postRedirectCheckForm(t, srv, map[string]string{"domain": "example.com"}, "r.csv", redirectCSV, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third check: status = %d, want 429", rec.Code)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "redirect checks already running") {
		t.Fatalf("error = %q", msg)
	}
}

func TestGetUnknownRedirectCheckIs404(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := doJSON(t, srv, http.MethodGet, "/api/redirect-checks/nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
