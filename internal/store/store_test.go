package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

func rep(seed, started string, pages int) *report.Report {
	return &report.Report{
		Seed:         seed,
		StartedAt:    started,
		FinishedAt:   started,
		PagesCrawled: pages,
		Summary:      report.Summary{BySeverity: map[string]int{"error": 1}},
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	s := New(t.TempDir())
	id, err := s.Save(rep("https://example.com", "2026-06-30T12:00:00Z", 7))
	if err != nil {
		t.Fatal(err)
	}
	if id != "example.com/20260630T120000Z" {
		t.Fatalf("id = %q, want example.com/20260630T120000Z", id)
	}
	got, err := s.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.PagesCrawled != 7 || got.Seed != "https://example.com" {
		t.Fatalf("loaded report mismatch: %+v", got)
	}
}

func TestListNewestFirstAndHostFilter(t *testing.T) {
	s := New(t.TempDir())
	mustSave(t, s, rep("https://a.com", "2026-06-01T00:00:00Z", 1))
	mustSave(t, s, rep("https://a.com", "2026-06-03T00:00:00Z", 1))
	mustSave(t, s, rep("https://b.com", "2026-06-02T00:00:00Z", 1))

	all, err := s.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("List all = %d, want 3", len(all))
	}
	if all[0].ID != "a.com/20260603T000000Z" {
		t.Fatalf("newest first failed, got %q", all[0].ID)
	}

	aOnly, err := s.List("a.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(aOnly) != 2 {
		t.Fatalf("host filter = %d, want 2", len(aOnly))
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	mustSave(t, s, rep("https://a.com", "2026-06-01T00:00:00Z", 1))
	id2 := mustSave(t, s, rep("https://a.com", "2026-06-03T00:00:00Z", 9))

	// latest
	if r, gotID, err := s.Resolve("latest"); err != nil || gotID != id2 || r.PagesCrawled != 9 {
		t.Fatalf("Resolve(latest) = %v, %q, %v", r, gotID, err)
	}
	// bare host -> latest for host
	if r, gotID, err := s.Resolve("a.com"); err != nil || gotID != id2 || r.PagesCrawled != 9 {
		t.Fatalf("Resolve(a.com) = %v, %q, %v", r, gotID, err)
	}
	// explicit ID
	if r, gotID, err := s.Resolve(id2); err != nil || gotID != id2 || r.PagesCrawled != 9 {
		t.Fatalf("Resolve(id) = %v, %q, %v", r, gotID, err)
	}
	// unknown ID errors
	if _, _, err := s.Resolve("a.com/19000101T000000Z"); err == nil {
		t.Fatal("Resolve of unknown ID should error")
	}
	// unknown host errors
	if _, _, err := s.Resolve("nope.com"); err == nil {
		t.Fatal("Resolve of unknown host should error")
	}
}

func TestResolveFilePathWins(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "store"))
	// A standalone report file outside the store, addressed by path.
	path := filepath.Join(dir, "saved.json")
	if err := writeJSON(path, rep("https://file.test", "2026-06-30T00:00:00Z", 42)); err != nil {
		t.Fatal(err)
	}
	r, id, err := s.Resolve(path)
	if err != nil {
		t.Fatal(err)
	}
	if id != "" {
		t.Fatalf("file-path resolve should have empty ID, got %q", id)
	}
	if r.PagesCrawled != 42 {
		t.Fatalf("loaded wrong report: %+v", r)
	}
}

func TestListEmptyStore(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "does-not-exist-yet"))
	entries, err := s.List("")
	if err != nil {
		t.Fatalf("List on empty store should not error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("empty store should list nothing, got %d", len(entries))
	}
}

// TestLoadRejectsPathTraversal guards against a real arbitrary-file-read: an ID containing
// ".." must not be allowed to escape the store root via pathForID's filepath.Join.
func TestLoadRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	s := New(storeDir)
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A secret file one level above the store root.
	secret := filepath.Join(dir, "secret.json")
	if err := writeJSON(secret, rep("https://secret.test", "2026-06-30T00:00:00Z", 1)); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Load("../secret"); err == nil {
		t.Fatal("expected Load to reject a traversal ID, got nil error")
	}
	if _, _, err := s.Resolve("../secret"); err == nil {
		t.Fatal("expected Resolve to reject a traversal ID, got nil error")
	}
}

// TestSaveRejectsHostileHost guards against a hostile seed URL whose "host" is literally ".."
// (Go's net/url will parse one), which would otherwise let Save's directory join escape the
// store root by one level.
func TestSaveRejectsHostileHost(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	s := New(storeDir)
	_, err := s.Save(rep("https://../evil", "2026-06-30T00:00:00Z", 1))
	if err == nil {
		t.Fatal("expected Save to reject a seed whose host escapes the store root, got nil error")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); !os.IsNotExist(err) {
		t.Error("Save must not have written outside the store root")
	}
}

func mustSave(t *testing.T, s *Store, r *report.Report) string {
	t.Helper()
	id, err := s.Save(r)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func writeJSON(path string, r *report.Report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
