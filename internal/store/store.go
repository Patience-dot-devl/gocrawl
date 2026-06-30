// Package store persists crawl reports to disk and reads them back, so crawls can be listed,
// reopened, and compared over time. A report is already a complete, self-contained JSON
// artifact (see internal/report), so the store is a thin filesystem layer over it: one JSON
// file per crawl, laid out by host and timestamp.
//
// Layout:
//
//	<root>/<host>/<timestamp>.json
//
// e.g. ~/.gocrawl/crawls/example.com/20260630T120000Z.json. The crawl ID is the
// "<host>/<timestamp>" stem — sortable (newest = lexicographically greatest) and readable.
package store

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// idTimeLayout formats the timestamp segment of a crawl ID: filesystem-safe (no colons) and
// lexicographically sortable so the newest crawl is the greatest ID.
const idTimeLayout = "20060102T150405Z"

// Store reads and writes crawl reports under a root directory.
type Store struct {
	root string
}

// DefaultRoot is the store directory used when none is configured: ~/.gocrawl/crawls, or
// ./.gocrawl/crawls if the home directory can't be determined.
func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".gocrawl", "crawls")
	}
	return filepath.Join(home, ".gocrawl", "crawls")
}

// New returns a Store rooted at dir. An empty dir uses DefaultRoot.
func New(dir string) *Store {
	if dir == "" {
		dir = DefaultRoot()
	}
	return &Store{root: dir}
}

// Root returns the store's root directory.
func (s *Store) Root() string { return s.root }

// Entry is the metadata of one saved crawl, cheap to list without rendering the full report.
type Entry struct {
	ID           string         `json:"id"`
	Host         string         `json:"host"`
	Seed         string         `json:"seed"`
	FinishedAt   string         `json:"finished_at"`
	PagesCrawled int            `json:"pages_crawled"`
	BySeverity   map[string]int `json:"by_severity"`
	Path         string         `json:"path"`
}

// Save writes rep to the store and returns its crawl ID. The host comes from the report's
// seed and the timestamp from its StartedAt (falling back to FinishedAt, then now).
func (s *Store) Save(rep *report.Report) (string, error) {
	host := hostOf(rep.Seed)
	if host == "" {
		host = "unknown-host"
	}
	ts := stamp(rep)
	dir := filepath.Join(s.root, host)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating store dir %q: %w", dir, err)
	}
	path := filepath.Join(dir, ts+".json")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	writeErr := enc.Encode(rep)
	closeErr := f.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return host + "/" + ts, nil
}

// Load reads the report stored under the given crawl ID ("<host>/<timestamp>").
func (s *Store) Load(id string) (*report.Report, error) {
	return readReport(s.pathForID(id))
}

func (s *Store) pathForID(id string) string {
	return filepath.Join(s.root, filepath.FromSlash(id)+".json")
}

// List returns every saved crawl, newest first. An optional host filters to one site
// (matched against the report seed's host); empty lists all.
func (s *Store) List(host string) ([]Entry, error) {
	var entries []Entry
	err := filepath.WalkDir(s.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // an empty/absent store is not an error
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		rep, rerr := readReport(path)
		if rerr != nil {
			return nil // skip unreadable / non-report files rather than failing the whole list
		}
		if host != "" && hostOf(rep.Seed) != host {
			return nil
		}
		entries = append(entries, entryFor(s.root, path, rep))
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Newest first by crawl time; the ID (host/timestamp) is a stable tiebreaker for crawls
	// that finished at the same instant.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].FinishedAt != entries[j].FinishedAt {
			return entries[i].FinishedAt > entries[j].FinishedAt
		}
		return entries[i].ID > entries[j].ID
	})
	return entries, nil
}

// Resolve turns a user-supplied reference into a report. A ref may be:
//   - a path to an existing JSON file (loaded directly, not from the store);
//   - "latest" — the newest crawl in the store;
//   - a bare host like "example.com" — that host's newest crawl;
//   - a crawl ID "<host>/<timestamp>".
//
// It returns the resolved report and the ID it resolved to (empty for a direct file path).
func (s *Store) Resolve(ref string) (*report.Report, string, error) {
	if ref == "" {
		return nil, "", fmt.Errorf("empty crawl reference")
	}
	// A real file on disk wins, so explicit paths always work regardless of the store.
	if fi, err := os.Stat(ref); err == nil && !fi.IsDir() {
		rep, rerr := readReport(ref)
		return rep, "", rerr
	}
	if strings.EqualFold(ref, "latest") {
		return s.latest("")
	}
	// A "<host>/<timestamp>" ID addresses a stored file directly.
	if strings.Contains(ref, "/") {
		if fi, err := os.Stat(s.pathForID(ref)); err == nil && !fi.IsDir() {
			rep, rerr := s.Load(ref)
			return rep, ref, rerr
		}
		return nil, "", fmt.Errorf("no stored crawl with ID %q (looked in %s)", ref, s.root)
	}
	// Otherwise treat it as a host name and take that site's newest crawl.
	return s.latest(ref)
}

func (s *Store) latest(host string) (*report.Report, string, error) {
	entries, err := s.List(host)
	if err != nil {
		return nil, "", err
	}
	if len(entries) == 0 {
		where := "the store"
		if host != "" {
			where = fmt.Sprintf("host %q", host)
		}
		return nil, "", fmt.Errorf("no saved crawls for %s (in %s)", where, s.root)
	}
	rep, rerr := readReport(entries[0].Path)
	return rep, entries[0].ID, rerr
}

func entryFor(root, path string, rep *report.Report) Entry {
	rel, err := filepath.Rel(root, path)
	id := strings.TrimSuffix(filepath.ToSlash(rel), ".json")
	if err != nil {
		id = strings.TrimSuffix(filepath.Base(path), ".json")
	}
	return Entry{
		ID:           id,
		Host:         hostOf(rep.Seed),
		Seed:         rep.Seed,
		FinishedAt:   rep.FinishedAt,
		PagesCrawled: rep.PagesCrawled,
		BySeverity:   rep.Summary.BySeverity,
		Path:         path,
	}
}

func readReport(path string) (*report.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rep report.Report
	if err := json.Unmarshal(data, &rep); err != nil {
		return nil, fmt.Errorf("parsing report %q as JSON: %w", path, err)
	}
	return &rep, nil
}

// hostOf extracts the host from a seed URL, tolerating a missing scheme.
func hostOf(seed string) string {
	if seed == "" {
		return ""
	}
	if !strings.Contains(seed, "://") {
		seed = "https://" + seed
	}
	u, err := url.Parse(seed)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// stamp returns the timestamp segment for a report's ID, derived from StartedAt then
// FinishedAt. An unparseable/empty time yields a zero-value stamp so saving never fails on a
// bad timestamp.
func stamp(rep *report.Report) string {
	for _, candidate := range []string{rep.StartedAt, rep.FinishedAt} {
		if t, err := time.Parse(time.RFC3339, candidate); err == nil {
			return t.UTC().Format(idTimeLayout)
		}
	}
	return time.Time{}.UTC().Format(idTimeLayout)
}
