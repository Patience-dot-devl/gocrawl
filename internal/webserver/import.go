package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// maxImportBytes caps the POST /api/reports/import request body. A report for a large site
// (every issue plus the site map) runs to tens of megabytes, so the cap is generous but
// bounded. It is a var so tests can lower it.
var maxImportBytes int64 = 64 << 20

// importClock supplies the time an imported report without timestamps is filed under. It is
// a var so tests can step it.
var importClock = time.Now

func (s *Server) importRoutes() {
	s.mux.HandleFunc("POST /api/reports/import", s.handleImportReport)
}

// handleImportReport saves an uploaded gocrawl JSON report to the store. This is the web
// equivalent of `gocrawl render`. The imported crawl then appears in the Crawls list and
// works with export and compare.
func (s *Server) handleImportReport(w http.ResponseWriter, r *http.Request) {
	if !parseMultipart(w, r, maxImportBytes) {
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New(`missing report file: send it as the multipart field "file"`))
		return
	}
	defer func() { _ = file.Close() }()

	var rep report.Report
	if err := json.NewDecoder(file).Decode(&rep); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("not a gocrawl JSON report: %w", err))
		return
	}
	if err := validateImportedSeed(rep.Seed); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	fillImportedDefaults(&rep)

	id, err := s.store.Save(&rep)
	if err != nil {
		// The error from the store names filesystem paths, so it stays out of the response.
		writeError(w, http.StatusInternalServerError, errors.New("could not save the imported report: check that the crawl store is writable and try again"))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// validateImportedSeed requires an http(s) seed URL with a host that is safe to use as a
// directory name. The store files a report under its seed's host, and every report gocrawl
// writes has such a seed.
func validateImportedSeed(seed string) error {
	if strings.TrimSpace(seed) == "" {
		return errors.New("the file has no seed URL; only JSON reports written by gocrawl can be imported")
	}
	u, err := url.Parse(seed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("the report's seed %q is not an http(s) URL", seed)
	}
	if !validHostComponent(u.Hostname()) {
		return fmt.Errorf("the report's seed %q does not have a valid host name", seed)
	}
	return nil
}

// validHostComponent reports whether host can be used as a single store directory name. It is
// a denylist so that Unicode (IDN) hosts, which the store files under their Unicode form, stay
// importable: it rejects path separators, NUL, control characters, whitespace, and hosts made
// of dots alone, which would name the directory itself or its parent.
func validHostComponent(host string) bool {
	if strings.Trim(host, ".") == "" {
		return false
	}
	for _, c := range host {
		if c == '/' || c == '\\' || c == 0 || unicode.IsControl(c) || unicode.IsSpace(c) {
			return false
		}
	}
	return true
}

// fillImportedDefaults gives an imported report the non-nil collections that a report built by
// report.Build always has, so the UI can read the summary maps and the issue list without null
// checks. It also fills a missing or zero started_at / finished_at: the store files a report
// under its start time, and a zero time would make every such import land on, and overwrite,
// the same ID.
func fillImportedDefaults(rep *report.Report) {
	if rep.Issues == nil {
		rep.Issues = []analyze.Issue{}
	}
	if rep.Summary.BySeverity == nil {
		rep.Summary.BySeverity = map[string]int{}
	}
	if rep.Summary.ByAnalyzer == nil {
		rep.Summary.ByAnalyzer = map[string]int{}
	}
	if rep.Summary.ByStatus == nil {
		rep.Summary.ByStatus = map[string]int{}
	}
	// A report with one valid timestamp reuses it for the other, so started_at never lands
	// after finished_at; one with neither is stamped with the import time.
	started, finished := hasTimestamp(rep.StartedAt), hasTimestamp(rep.FinishedAt)
	switch {
	case !started && !finished:
		now := importClock().UTC().Format(time.RFC3339)
		rep.StartedAt, rep.FinishedAt = now, now
	case !started:
		rep.StartedAt = rep.FinishedAt
	case !finished:
		rep.FinishedAt = rep.StartedAt
	}
}

// hasTimestamp reports whether s is a real RFC 3339 time, rather than empty, unparseable, or
// the zero time.
func hasTimestamp(s string) bool {
	t, err := time.Parse(time.RFC3339, s)
	return err == nil && !t.IsZero()
}
