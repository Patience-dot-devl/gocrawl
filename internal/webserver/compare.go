package webserver

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Patience-dot-devl/gocrawl/internal/diff"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// errCompareUnfinished marks a compare ref that names a crawl job still in progress, which
// has no report to compare yet.
var errCompareUnfinished = errors.New("crawl has not finished yet")

func (s *Server) compareRoutes() {
	s.mux.HandleFunc("GET /api/compare", s.handleCompare)
}

// handleCompare diffs two crawls, base being the earlier one, with the same diff.Compare the
// `gocrawl compare` command prints.
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	baseRef, currentRef := q.Get("base"), q.Get("current")
	if baseRef == "" || currentRef == "" {
		writeError(w, http.StatusBadRequest, errors.New(`both "base" and "current" are required: a crawl ID, a host such as example.com, or "latest"`))
		return
	}
	base, err := s.resolveCompareRef(baseRef)
	if err != nil {
		writeCompareRefError(w, "base", err)
		return
	}
	current, err := s.resolveCompareRef(currentRef)
	if err != nil {
		writeCompareRefError(w, "current", err)
		return
	}
	writeJSON(w, http.StatusOK, diff.Compare(base, current))
}

// resolveCompareRef finds the report a compare ref names. A crawl-job ID from this process
// wins, because GET /api/crawls lists those jobs under their job ID. Otherwise the ref goes
// to the store: a crawl ID, a bare host, or "latest". File paths are deliberately not
// accepted (store.ResolveStored).
//
// A store failure is reported as a generic "not found": the store's own error names its root
// directory, which a network client has no business seeing.
func (s *Server) resolveCompareRef(ref string) (*report.Report, error) {
	if job, ok := s.crawls.get(ref); ok {
		if job.Data.Report != nil {
			return job.Data.Report, nil
		}
		if job.Status == StatusRunning || job.Status == StatusPaused {
			return nil, fmt.Errorf("crawl %q: %w", ref, errCompareUnfinished)
		}
		return nil, fmt.Errorf("crawl %q has no report (status %s)", ref, job.Status)
	}
	rep, _, err := s.store.ResolveStored(ref)
	if err != nil {
		return nil, fmt.Errorf("crawl %q not found; pass a crawl ID, a host such as example.com, or \"latest\"", ref)
	}
	return rep, nil
}

func writeCompareRefError(w http.ResponseWriter, side string, err error) {
	status := http.StatusNotFound
	if errors.Is(err, errCompareUnfinished) {
		status = http.StatusConflict
	}
	writeError(w, status, fmt.Errorf("%s: %w", side, err))
}
