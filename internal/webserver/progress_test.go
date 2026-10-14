package webserver

import (
	"net/http"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

func pageEvent(fetched, status int, inSitemap bool) runner.Progress {
	return runner.Progress{
		Progress:     crawler.Progress{Fetched: fetched, Status: status, CurrentURL: "https://example.com/p"},
		Phase:        runner.PhaseCrawling,
		SitemapFound: true,
		SitemapTotal: 10,
		InSitemap:    inSitemap,
	}
}

func TestRecordProgressGrowsTapeAndSlicesByTapeFrom(t *testing.T) {
	srv := newTestServer(t)
	job, ok := srv.crawls.start(crawlJob{Seed: "https://example.com/", MaxPages: 500}, func() {}, srv.maxRunning)
	if !ok {
		t.Fatal("could not start a job")
	}
	srv.recordProgress(job.ID, runner.Progress{Phase: runner.PhaseSitemap}) // no page yet
	srv.recordProgress(job.ID, pageEvent(1, 200, true))
	srv.recordProgress(job.ID, pageEvent(2, 301, false))
	phaseOnly := pageEvent(2, 301, false) // same Fetched: a state event adds no tick
	phaseOnly.Phase = runner.PhaseAnalyzing
	srv.recordProgress(job.ID, phaseOnly)
	srv.recordProgress(job.ID, pageEvent(3, 404, true))
	srv.recordProgress(job.ID, pageEvent(4, 0, true))

	_, full := doJSON(t, srv, http.MethodGet, "/api/crawls/"+job.ID, nil)
	prog, ok := full["progress"].(map[string]any)
	if !ok {
		t.Fatalf("no progress in %v", full)
	}
	if prog["tape"] != "234x" || prog["tape_off"] != "0100" || prog["tape_from"] != float64(0) {
		t.Errorf("tape=%v tape_off=%v tape_from=%v, want 234x / 0100 / 0", prog["tape"], prog["tape_off"], prog["tape_from"])
	}
	if prog["max_pages"] != float64(500) || prog["fetched"] != float64(4) || prog["sitemap_total"] != float64(10) {
		t.Errorf("max_pages=%v fetched=%v sitemap_total=%v", prog["max_pages"], prog["fetched"], prog["sitemap_total"])
	}

	_, tail := doJSON(t, srv, http.MethodGet, "/api/crawls/"+job.ID+"?tape_from=2", nil)
	tp := tail["progress"].(map[string]any)
	if tp["tape"] != "4x" || tp["tape_off"] != "00" || tp["tape_from"] != float64(2) {
		t.Errorf("tape_from=2: tape=%v tape_off=%v tape_from=%v", tp["tape"], tp["tape_off"], tp["tape_from"])
	}

	_, past := doJSON(t, srv, http.MethodGet, "/api/crawls/"+job.ID+"?tape_from=99", nil)
	pp := past["progress"].(map[string]any)
	if pp["tape"] != "" || pp["tape_from"] != float64(4) {
		t.Errorf("tape_from past the end: tape=%v tape_from=%v, want empty / 4", pp["tape"], pp["tape_from"])
	}

	for _, bad := range []string{"-1", "abc"} {
		rec, _ := doJSON(t, srv, http.MethodGet, "/api/crawls/"+job.ID+"?tape_from="+bad, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("tape_from=%s: status %d, want 400", bad, rec.Code)
		}
	}
}

func TestRecordProgressWithoutSitemapNeverNotches(t *testing.T) {
	srv := newTestServer(t)
	job, _ := srv.crawls.start(crawlJob{Seed: "https://example.com/"}, func() {}, srv.maxRunning)
	ev := pageEvent(1, 200, false)
	ev.SitemapFound = false
	srv.recordProgress(job.ID, ev)
	_, out := doJSON(t, srv, http.MethodGet, "/api/crawls/"+job.ID, nil)
	if got := out["progress"].(map[string]any)["tape_off"]; got != "0" {
		t.Errorf("tape_off = %v, want 0 when no sitemap was found", got)
	}
}

func TestCrawlProgressEndToEnd(t *testing.T) {
	srv := newTestServer(t)
	target := startCrawlTarget(t)
	rec, started := doJSON(t, srv, http.MethodPost, "/api/crawls", map[string]any{"url": target.URL, "max_pages": 1})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, body %v", rec.Code, started)
	}
	id, _ := started["id"].(string)
	job := pollUntilFinished(t, srv, id)
	prog, ok := job["progress"].(map[string]any)
	if !ok {
		t.Fatalf("finished job has no progress: %v", job)
	}
	if prog["phase"] != "analyzing" || prog["tape"] != "2" || prog["fetched"] != float64(1) || prog["max_pages"] != float64(1) {
		t.Errorf("progress = %v", prog)
	}

	_, list := doJSON(t, srv, http.MethodGet, "/api/crawls", nil)
	items, _ := list["crawls"].([]any)
	if len(items) == 0 {
		t.Fatal("empty crawl list")
	}
	first, _ := items[0].(map[string]any)
	classes, _ := first["status_classes"].(map[string]any)
	if classes["2"] != float64(1) {
		t.Errorf("status_classes = %v, want {2: 1}", classes)
	}
	if first["tape_preview"] != "2" {
		t.Errorf("tape_preview = %v, want 2", first["tape_preview"])
	}
	if _, ok := first["tape"]; ok {
		t.Error("crawl list items must not carry the full tape")
	}
}

// After a server restart an in-memory job is gone. A poll for it (with tape_from, as the UI
// sends) must get a JSON 404, so the UI stops polling and says what happened, not a 400 or a
// hang.
func TestGetCrawlForgottenJobIs404(t *testing.T) {
	srv := newTestServer(t)
	rec, out := doJSON(t, srv, http.MethodGet, "/api/crawls/0123456789abcdef?tape_from=40", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
	if msg, _ := out["error"].(string); msg == "" {
		t.Errorf("body %v, want a JSON error", out)
	}
}

// A saved crawl (listed from the store, e.g. after a restart) shows the classes of its tape, the
// same ones the live tape showed, and falls back to pages_by_status only without a tape.
func TestListSavedCrawlStatusClasses(t *testing.T) {
	srv := newTestServer(t)
	withTape := &report.Report{
		Seed: "https://tape.example/", StartedAt: "2026-10-13T10:00:00Z", FinishedAt: "2026-10-13T10:00:00Z",
		Summary:  report.Summary{ByStatus: map[string]int{"200": 9}}, // disagrees on purpose
		Coverage: &crawler.Coverage{SitemapTotal: 4, SitemapFetched: 2, Tape: "224"},
	}
	legacy := &report.Report{
		Seed: "https://legacy.example/", StartedAt: "2026-10-13T11:00:00Z", FinishedAt: "2026-10-13T11:00:00Z",
		Summary: report.Summary{ByStatus: map[string]int{"200": 2, "404": 1, "0": 1}},
	}
	for _, r := range []*report.Report{withTape, legacy} {
		if _, err := srv.store.Save(r); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	_, list := doJSON(t, srv, http.MethodGet, "/api/crawls", nil)
	bySeed := map[string]map[string]any{}
	for _, it := range list["crawls"].([]any) {
		m := it.(map[string]any)
		bySeed[m["seed"].(string)] = m
	}
	tapeItem := bySeed["https://tape.example/"]
	classes, _ := tapeItem["status_classes"].(map[string]any)
	if classes["2"] != float64(2) || classes["4"] != float64(1) || len(classes) != 2 {
		t.Errorf("tape crawl status_classes = %v, want {2: 2, 4: 1}", classes)
	}
	if tapeItem["tape_preview"] != "224" || tapeItem["sitemap_total"] != float64(4) || tapeItem["sitemap_fetched"] != float64(2) {
		t.Errorf("tape crawl item = %v", tapeItem)
	}
	legacyClasses, _ := bySeed["https://legacy.example/"]["status_classes"].(map[string]any)
	if legacyClasses["2"] != float64(2) || legacyClasses["4"] != float64(1) || legacyClasses["x"] != float64(1) {
		t.Errorf("legacy status_classes = %v, want {2: 2, 4: 1, x: 1}", legacyClasses)
	}
	if _, ok := bySeed["https://legacy.example/"]["tape_preview"]; ok {
		t.Error("a report without a tape must not carry a tape_preview")
	}
}
