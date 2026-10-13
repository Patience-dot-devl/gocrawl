package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/sitemapgen"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

// compareFixture builds a minimal report of example.com: the given issues, plus a site map
// listing the given pages (diff.Compare takes page sets from the site map).
func compareFixture(started string, issues []analyze.Issue, pages ...string) *report.Report {
	rep := &report.Report{
		Seed:         "https://example.com/",
		StartedAt:    started,
		FinishedAt:   started,
		PagesCrawled: len(pages),
		Issues:       issues,
		Summary: report.Summary{
			BySeverity: map[string]int{},
			ByAnalyzer: map[string]int{},
			ByStatus:   map[string]int{"200": len(pages)},
		},
		SiteMap: &sitemapgen.Map{Seed: "https://example.com/"},
	}
	for _, is := range issues {
		rep.Summary.BySeverity[string(is.Severity)]++
		rep.Summary.ByAnalyzer[is.Analyzer]++
	}
	for _, p := range pages {
		rep.SiteMap.Entries = append(rep.SiteMap.Entries, sitemapgen.Entry{Loc: p})
	}
	return rep
}

func compareIssue(code, pageURL string) analyze.Issue {
	return analyze.Issue{Analyzer: "seo", URL: pageURL, Severity: analyze.Error, Code: code, Message: code}
}

// saveComparePair stores an older and a newer crawl of example.com and returns their IDs. The
// newer one fixes missing-title, adds missing-description, drops /b and adds /c.
func saveComparePair(t *testing.T, srv *Server) (baseID, currentID string) {
	t.Helper()
	base := compareFixture("2026-10-01T10:00:00Z",
		[]analyze.Issue{compareIssue("missing-title", "https://example.com/a")},
		"https://example.com/a", "https://example.com/b")
	current := compareFixture("2026-10-08T10:00:00Z",
		[]analyze.Issue{compareIssue("missing-description", "https://example.com/a")},
		"https://example.com/a", "https://example.com/c")
	var err error
	if baseID, err = srv.store.Save(base); err != nil {
		t.Fatalf("save base: %v", err)
	}
	if currentID, err = srv.store.Save(current); err != nil {
		t.Fatalf("save current: %v", err)
	}
	return baseID, currentID
}

func compareURL(base, current string) string {
	q := url.Values{}
	if base != "" {
		q.Set("base", base)
	}
	if current != "" {
		q.Set("current", current)
	}
	return "/api/compare?" + q.Encode()
}

func assertCompareCodes(t *testing.T, label string, list any, want ...string) {
	t.Helper()
	items, _ := list.([]any)
	got := []string{}
	for _, it := range items {
		code, _ := it.(map[string]any)["code"].(string)
		got = append(got, code)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s codes = %v, want %v", label, got, want)
	}
}

func TestCompareDiffsTwoStoredCrawls(t *testing.T) {
	srv := newTestServer(t)
	baseID, currentID := saveComparePair(t, srv)

	// The current side can be an ID, a bare host, or "latest", as on the CLI.
	for _, current := range []string{currentID, "example.com", "latest"} {
		rec, out := doJSON(t, srv, http.MethodGet, compareURL(baseID, current), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("current=%q: status = %d, body = %s", current, rec.Code, rec.Body)
		}
		issues, _ := out["issues"].(map[string]any)
		assertCompareCodes(t, "new", issues["new"], "missing-description")
		assertCompareCodes(t, "resolved", issues["resolved"], "missing-title")
		pages, _ := out["pages"].(map[string]any)
		if got := fmt.Sprint(pages["added"]); got != "[https://example.com/c]" {
			t.Errorf("added = %s", got)
		}
		if got := fmt.Sprint(pages["removed"]); got != "[https://example.com/b]" {
			t.Errorf("removed = %s", got)
		}
		summary, _ := out["summary"].(map[string]any)
		if _, ok := summary["new_by_severity"].(map[string]any); !ok {
			t.Errorf("summary.new_by_severity missing: %v", summary)
		}
	}
}

func TestCompareRequiresBothRefs(t *testing.T) {
	srv := newTestServer(t)
	baseID, _ := saveComparePair(t, srv)
	for _, tc := range [][2]string{{"", ""}, {baseID, ""}, {"", baseID}} {
		rec, out := doJSON(t, srv, http.MethodGet, compareURL(tc[0], tc[1]), nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("base=%q current=%q: status = %d, want 400", tc[0], tc[1], rec.Code)
		}
		if msg, _ := out["error"].(string); msg == "" {
			t.Errorf("base=%q current=%q: no error message", tc[0], tc[1])
		}
	}
}

func TestCompareUnknownRefIs404(t *testing.T) {
	srv := newTestServer(t)
	_, currentID := saveComparePair(t, srv)
	for _, ref := range []string{"nope.example", "example.com/20990101T000000Z", "0123456789abcdef"} {
		rec, _ := doJSON(t, srv, http.MethodGet, compareURL(ref, currentID), nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("base=%q: status = %d, want 404", ref, rec.Code)
		}
	}
}

// A failed lookup must not reveal where the store lives, nor the text of the underlying error.
func TestCompareNotFoundDoesNotLeakStorePaths(t *testing.T) {
	root := t.TempDir()
	srv := New(store.New(root))
	_, currentID := saveComparePair(t, srv)
	for _, ref := range []string{"nope.example", "example.com/20990101T000000Z", "../../etc/passwd"} {
		for _, q := range []string{compareURL(ref, currentID), compareURL(currentID, ref)} {
			rec, out := doJSON(t, srv, http.MethodGet, q, nil)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("ref=%q: status = %d, want 404", ref, rec.Code)
			}
			body := rec.Body.String()
			for _, leak := range []string{root, filepath.Base(root), "looked in", "no saved crawls", "no stored crawl"} {
				if strings.Contains(body, leak) {
					t.Errorf("ref=%q: body %q leaks %q", ref, body, leak)
				}
			}
			if msg, _ := out["error"].(string); !strings.Contains(msg, "not found") {
				t.Errorf("ref=%q: error = %q, want a not-found message", ref, msg)
			}
		}
	}
}

func TestCompareNeverReadsFilePaths(t *testing.T) {
	srv := newTestServer(t)
	_, currentID := saveComparePair(t, srv)
	outside := filepath.Join(t.TempDir(), "report.json")
	data, err := json.Marshal(compareFixture("2026-01-01T00:00:00Z", nil, "https://example.com/x"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, data, 0o600); err != nil {
		t.Fatal(err)
	}
	rec, _ := doJSON(t, srv, http.MethodGet, compareURL(outside, currentID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("file-path ref: status = %d, want 404", rec.Code)
	}
}

func TestCompareUsesCrawlJobsFromThisProcess(t *testing.T) {
	srv := newTestServer(t)
	baseID, _ := saveComparePair(t, srv)

	job, ok := srv.crawls.start(crawlJob{Seed: "https://example.com/"}, func() {}, DefaultMaxRunning)
	if !ok {
		t.Fatal("could not start a fake crawl job")
	}
	rec, _ := doJSON(t, srv, http.MethodGet, compareURL(baseID, job.ID), nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("running job: status = %d, want 409", rec.Code)
	}

	srv.crawls.finish(job.ID, nil, func(d *crawlJob) {
		d.Report = compareFixture("2026-10-09T10:00:00Z", nil, "https://example.com/a")
	})
	rec, out := doJSON(t, srv, http.MethodGet, compareURL(baseID, job.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("finished job: status = %d, body = %s", rec.Code, rec.Body)
	}
	issues, _ := out["issues"].(map[string]any)
	assertCompareCodes(t, "resolved", issues["resolved"], "missing-title")

	failed, ok := srv.crawls.start(crawlJob{Seed: "https://example.com/"}, func() {}, DefaultMaxRunning)
	if !ok {
		t.Fatal("could not start a second fake crawl job")
	}
	srv.crawls.finish(failed.ID, errors.New("boom"), nil)
	rec, _ = doJSON(t, srv, http.MethodGet, compareURL(baseID, failed.ID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("failed job: status = %d, want 404", rec.Code)
	}
}
