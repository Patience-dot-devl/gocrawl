package webserver

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

const importFixture = `{"seed":"https://example.com/","started_at":"2026-10-13T10:00:00Z",` +
	`"finished_at":"2026-10-13T10:01:00Z","pages_crawled":2,` +
	`"summary":{"by_severity":{"error":1},"by_analyzer":{"seo":1},"pages_by_status":{"200":2}},` +
	`"issues":[{"analyzer":"seo","url":"https://example.com/","severity":"error","code":"missing-title","message":"Page has no title"}]}`

// postImportFile sends body as the multipart "file" part of POST /api/reports/import through
// the full handler, guard included. An empty fileName sends a form with no file part.
func postImportFile(t *testing.T, srv *Server, fileName string, body []byte, origin string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if fileName == "" {
		if err := mw.WriteField("note", "no file here"); err != nil {
			t.Fatal(err)
		}
	} else {
		fw, err := mw.CreateFormFile("file", fileName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/reports/import", &buf)
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

func storedCount(t *testing.T, srv *Server) int {
	t.Helper()
	entries, err := srv.store.List("")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestImportReportSavesIt(t *testing.T) {
	srv := newTestServer(t)
	rec, out := postImportFile(t, srv, "report.json", []byte(importFixture), "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	id, _ := out["id"].(string)
	if id != "example.com/20261013T100000Z" {
		t.Fatalf("id = %q", id)
	}

	rec, job := doJSON(t, srv, http.MethodGet, "/api/crawls/"+url.PathEscape(id), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET imported crawl: status = %d", rec.Code)
	}
	rep, _ := job["report"].(map[string]any)
	if rep["seed"] != "https://example.com/" {
		t.Fatalf("imported report seed = %v", rep["seed"])
	}

	_, list := doJSON(t, srv, http.MethodGet, "/api/crawls", nil)
	crawls, _ := list["crawls"].([]any)
	found := false
	for _, c := range crawls {
		if c.(map[string]any)["id"] == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("imported crawl %q missing from GET /api/crawls: %v", id, crawls)
	}
}

func TestImportReportRejectsInvalidFiles(t *testing.T) {
	srv := newTestServer(t)
	for name, body := range map[string]string{
		"not JSON":            "this is not json",
		"JSON array":          "[1,2]",
		"no seed":             `{"pages_crawled":3}`,
		"null":                "null",
		"non-http seed":       `{"seed":"file:///etc/passwd"}`,
		"seed with no host":   `{"seed":"https://"}`,
		"dot host":            `{"seed":"https://./"}`,
		"dotdot host":         `{"seed":"https://../"}`,
		"host with slash":     `{"seed":"https://a%2Fb/"}`,
		"host with space":     `{"seed":"https://a b/"}`,
		"host with backslash": `{"seed":"https://a\\b/"}`,
	} {
		rec, out := postImportFile(t, srv, "report.json", []byte(body), "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
		if msg, _ := out["error"].(string); msg == "" {
			t.Errorf("%s: no error message", name)
		}
	}
	if n := storedCount(t, srv); n != 0 {
		t.Fatalf("%d report(s) saved from invalid uploads", n)
	}
}

func TestImportReportRequiresFileField(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := postImportFile(t, srv, "", nil, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestImportReportRejectsOversizedUpload(t *testing.T) {
	prev := maxImportBytes
	maxImportBytes = 1024
	t.Cleanup(func() { maxImportBytes = prev })

	srv := newTestServer(t)
	big := importFixture + strings.Repeat(" ", 4096)
	rec, _ := postImportFile(t, srv, "report.json", []byte(big), "")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if n := storedCount(t, srv); n != 0 {
		t.Fatalf("%d report(s) saved from an oversized upload", n)
	}
}

func TestImportReportRequiresMultipart(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := doJSON(t, srv, http.MethodPost, "/api/reports/import", map[string]any{"seed": "https://example.com/"})
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestImportReportRejectsCrossOrigin(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := postImportFile(t, srv, "report.json", []byte(importFixture), "http://evil.example")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if n := storedCount(t, srv); n != 0 {
		t.Fatalf("%d report(s) saved despite the cross-origin rejection", n)
	}
}

func TestImportReportFillsEmptyCollections(t *testing.T) {
	srv := newTestServer(t)
	rec, out := postImportFile(t, srv, "minimal.json", []byte(`{"seed":"https://example.com/","started_at":"2026-10-13T10:00:00Z"}`), "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	_, job := doJSON(t, srv, http.MethodGet, "/api/crawls/"+url.PathEscape(out["id"].(string)), nil)
	rep, _ := job["report"].(map[string]any)
	summary, _ := rep["summary"].(map[string]any)
	for _, key := range []string{"by_severity", "by_analyzer", "pages_by_status"} {
		if _, ok := summary[key].(map[string]any); !ok {
			t.Errorf("summary.%s = %v, want an object (the UI reads it without null checks)", key, summary[key])
		}
	}
	if issues, ok := rep["issues"].([]any); !ok || len(issues) != 0 {
		t.Errorf("issues = %v, want []", rep["issues"])
	}
}

func TestImportReportFillsMissingTimestamps(t *testing.T) {
	// A clock that moves one second per reading, so imports can't collide on the ID's
	// one-second resolution.
	tick := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	prevClock := importClock
	importClock = func() time.Time { tick = tick.Add(time.Second); return tick }
	t.Cleanup(func() { importClock = prevClock })

	srv := newTestServer(t)
	for name, body := range map[string]string{
		"absent": `{"seed":"https://example.com/"}`,
		"empty":  `{"seed":"https://example.com/","started_at":"","finished_at":""}`,
		"zero":   `{"seed":"https://example.com/","started_at":"0001-01-01T00:00:00Z","finished_at":"0001-01-01T00:00:00Z"}`,
	} {
		rec, out := postImportFile(t, srv, "report.json", []byte(body), "")
		if rec.Code != http.StatusCreated {
			t.Fatalf("%s: status = %d, body = %s", name, rec.Code, rec.Body)
		}
		id, _ := out["id"].(string)
		if strings.Contains(id, "00010101") {
			t.Errorf("%s: id %q carries the zero timestamp", name, id)
		}
		_, job := doJSON(t, srv, http.MethodGet, "/api/crawls/"+url.PathEscape(id), nil)
		rep, _ := job["report"].(map[string]any)
		for _, key := range []string{"started_at", "finished_at"} {
			ts, err := time.Parse(time.RFC3339, rep[key].(string))
			if err != nil || ts.Year() != 2026 || ts.Month() != 10 || ts.Day() != 14 {
				t.Errorf("%s: %s = %v, want the import time", name, key, rep[key])
			}
		}
	}
	if n := storedCount(t, srv); n != 3 {
		t.Fatalf("%d report(s) stored, want 3 (imports without timestamps must not overwrite each other)", n)
	}
}

func TestImportReportKeepsGivenTimestamps(t *testing.T) {
	rep := report.Report{Seed: "https://example.com/", StartedAt: "2026-10-13T10:00:00Z", FinishedAt: "2026-10-13T10:01:00Z"}
	fillImportedDefaults(&rep)
	if rep.StartedAt != "2026-10-13T10:00:00Z" || rep.FinishedAt != "2026-10-13T10:01:00Z" {
		t.Fatalf("timestamps changed: %q, %q", rep.StartedAt, rep.FinishedAt)
	}
}

func TestImportReportWritesNothingOutsideTheHostDirectory(t *testing.T) {
	dir := t.TempDir()
	srv := New(store.New(dir))
	for _, seed := range []string{"https://./", "https://../", "https://.../", `https://a\\b/`} {
		rec, _ := postImportFile(t, srv, "report.json", []byte(`{"seed":"`+seed+`","started_at":"2026-10-13T10:00:00Z"}`), "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", seed, rec.Code)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("store root holds %d entries after rejected imports: %v", len(entries), entries)
	}
}

func TestImportReportRemovesSpilledUploadParts(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	srv := newTestServer(t)
	// Larger than multipartMemory, so the file part is spilled to a temp file on disk.
	big := importFixture + strings.Repeat(" ", multipartMemory+1024)
	rec, _ := postImportFile(t, srv, "report.json", []byte(big), "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %.200s", rec.Code, rec.Body)
	}
	left, err := filepath.Glob(filepath.Join(tmp, "multipart-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("spilled upload parts left behind: %v", left)
	}
}

func TestImportReportErrorsDoNotLeakThePath(t *testing.T) {
	dir := t.TempDir()
	// A file where the host directory should go makes Save fail with an OS error that names the path.
	if err := os.WriteFile(filepath.Join(dir, "example.com"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(store.New(dir))
	rec, out := postImportFile(t, srv, "report.json", []byte(importFixture), "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body)
	}
	if msg, _ := out["error"].(string); msg == "" || strings.Contains(rec.Body.String(), dir) || strings.Contains(rec.Body.String(), "example.com/") {
		t.Fatalf("error %q is empty or leaks the store path %q", msg, dir)
	}
}

func TestImportReportAcceptsUnicodeAndPortHosts(t *testing.T) {
	for name, tc := range map[string]struct{ seed, wantHost string }{
		"Devanagari":     {"https://भारत.com/", "भारत.com"},
		"combining mark": {"https://e\u0301.com/", "e\u0301.com"},
		"precomposed":    {"https://é.com/", "é.com"},
		"punycode":       {"https://xn--bcher-kva.example/", "xn--bcher-kva.example"},
		"port":           {"https://xn--bcher-kva.example:8443/", "xn--bcher-kva.example"},
	} {
		srv := newTestServer(t)
		body := `{"seed":"` + tc.seed + `","started_at":"2026-10-13T10:00:00Z"}`
		rec, out := postImportFile(t, srv, "report.json", []byte(body), "")
		if rec.Code != http.StatusCreated {
			t.Errorf("%s: status = %d, body = %s", name, rec.Code, rec.Body)
			continue
		}
		if id, _ := out["id"].(string); id != tc.wantHost+"/20261013T100000Z" {
			t.Errorf("%s: id = %q, want host %q without a port", name, id, tc.wantHost)
		}
	}
}

func TestImportReportFillsOneMissingTimestampFromTheOther(t *testing.T) {
	rep := report.Report{Seed: "https://example.com/", FinishedAt: "2026-10-13T10:01:00Z"}
	fillImportedDefaults(&rep)
	if rep.StartedAt != "2026-10-13T10:01:00Z" || rep.FinishedAt != "2026-10-13T10:01:00Z" {
		t.Errorf("only finished_at given: started_at = %q, finished_at = %q", rep.StartedAt, rep.FinishedAt)
	}
	rep = report.Report{Seed: "https://example.com/", StartedAt: "2026-10-13T10:00:00Z", FinishedAt: "0001-01-01T00:00:00Z"}
	fillImportedDefaults(&rep)
	if rep.StartedAt != "2026-10-13T10:00:00Z" || rep.FinishedAt != "2026-10-13T10:00:00Z" {
		t.Errorf("only started_at given: started_at = %q, finished_at = %q", rep.StartedAt, rep.FinishedAt)
	}
}
