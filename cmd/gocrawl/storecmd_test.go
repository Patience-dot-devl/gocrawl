package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/diff"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

const baseReportJSON = `{
  "seed": "https://example.com",
  "started_at": "2026-06-01T10:00:00Z",
  "finished_at": "2026-06-01T10:01:00Z",
  "pages_crawled": 1,
  "summary": {"by_severity":{"error":1},"by_analyzer":{"seo":1},"pages_by_status":{"200":1}},
  "issues": [
    {"analyzer":"seo","url":"https://example.com/a","severity":"error","code":"seo-missing-h1","message":"No H1"}
  ],
  "site_map": {"seed":"https://example.com","host":"example.com","entries":[{"loc":"https://example.com/a"}]}
}`

const currentReportJSON = `{
  "seed": "https://example.com",
  "started_at": "2026-06-08T10:00:00Z",
  "finished_at": "2026-06-08T10:01:00Z",
  "pages_crawled": 1,
  "summary": {"by_severity":{"warning":1},"by_analyzer":{"links":1},"pages_by_status":{"200":1}},
  "issues": [
    {"analyzer":"links","url":"https://example.com/b","severity":"warning","code":"broken-link","message":"404"}
  ],
  "site_map": {"seed":"https://example.com","host":"example.com","entries":[{"loc":"https://example.com/b"}]}
}`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestCompareCmdJSON(t *testing.T) {
	dir := t.TempDir()
	base := writeFile(t, dir, "base.json", baseReportJSON)
	current := writeFile(t, dir, "current.json", currentReportJSON)
	out := filepath.Join(dir, "diff.json")

	cmd := newCompareCmd()
	cmd.SetArgs([]string{base, current, "-f", "json", "-o", out})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compare: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var d diff.Diff
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("diff JSON: %v", err)
	}
	if len(d.Issues.New) != 1 || d.Issues.New[0].Code != "broken-link" {
		t.Errorf("New = %+v, want broken-link", d.Issues.New)
	}
	if len(d.Issues.Resolved) != 1 || d.Issues.Resolved[0].Code != "seo-missing-h1" {
		t.Errorf("Resolved = %+v, want seo-missing-h1", d.Issues.Resolved)
	}
	if len(d.Pages.Added) != 1 || len(d.Pages.Removed) != 1 {
		t.Errorf("page diff = +%v -%v, want one each", d.Pages.Added, d.Pages.Removed)
	}
}

func TestCompareCmdFailOnNew(t *testing.T) {
	dir := t.TempDir()
	base := writeFile(t, dir, "base.json", baseReportJSON)
	current := writeFile(t, dir, "current.json", currentReportJSON)

	cmd := newCompareCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{base, current, "-o", filepath.Join(dir, "out.txt"), "--fail-on-new"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected non-zero exit when new issues are introduced")
	}
}

func TestHistoryCmdJSON(t *testing.T) {
	storeDir := t.TempDir()
	// Seed the store the same way `gocrawl crawl --save` does.
	saveViaCmd(t, storeDir, baseReportJSON)
	saveViaCmd(t, storeDir, currentReportJSON)

	cmd := newHistoryCmd()
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--store-dir", storeDir, "-f", "json"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("history: %v", err)
		}
	})

	var entries []store.Entry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("history JSON: %v\n%s", err, out)
	}
	if len(entries) != 2 {
		t.Fatalf("history listed %d entries, want 2", len(entries))
	}
	// Newest first.
	if !strings.HasPrefix(entries[0].ID, "example.com/20260608") {
		t.Errorf("newest-first failed: %q", entries[0].ID)
	}
}

// saveViaCmd writes a report JSON into the store by parsing it and calling Save, mirroring
// what `gocrawl crawl --save` does.
func saveViaCmd(t *testing.T, storeDir, reportJSON string) {
	t.Helper()
	tmp := writeFile(t, t.TempDir(), "r.json", reportJSON)
	s := store.New(storeDir)
	rep, _, err := s.Resolve(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(rep); err != nil {
		t.Fatal(err)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	_ = w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	return string(data)
}
