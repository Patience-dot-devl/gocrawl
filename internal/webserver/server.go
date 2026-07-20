// Package webserver exposes gocrawl over HTTP: an API to start and inspect crawls, backed by
// the same runner.Run seam the CLI and MCP server use, plus the embedded single-page app
// (see assets.go) that drives it from a browser.
package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/Patience-dot-devl/gocrawl/internal/crawlrequest"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

// Server serves the gocrawl web API and embedded frontend.
type Server struct {
	store *store.Store
	jobs  *jobManager
	mux   *http.ServeMux
}

// New builds a Server backed by st for crawl history (persisted via --save).
func New(st *store.Store) *Server {
	s := &Server{
		store: st,
		jobs:  newJobManager(),
	}
	s.mux = http.NewServeMux()
	s.routes()
	return s
}

// Handler returns the http.Handler serving the API and frontend.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/analyzers", s.handleListAnalyzers)
	s.mux.HandleFunc("POST /api/crawls", s.handleStartCrawl)
	s.mux.HandleFunc("GET /api/crawls", s.handleListCrawls)
	s.mux.HandleFunc("GET /api/crawls/{id}", s.handleGetCrawl)
	s.mux.HandleFunc("POST /api/crawls/{id}/cancel", s.handleCancelCrawl)
	s.mux.HandleFunc("GET /api/crawls/{id}/export", s.handleExportCrawl)
	s.mux.Handle("/", frontendHandler())
}

func (s *Server) handleListAnalyzers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"analyzers": runner.ListAnalyzers()})
}

// startCrawlRequest is the POST /api/crawls body: the shared crawl params plus web-only
// options.
type startCrawlRequest struct {
	crawlrequest.Params
	Save bool `json:"save"`
}

func (s *Server) handleStartCrawl(w http.ResponseWriter, r *http.Request) {
	var req startCrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	cfg, seed, err := req.ToConfig()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := s.jobs.start(seed, cancel)

	go func() {
		defer cancel()
		rep, runErr := runner.Run(ctx, cfg, seed)
		s.jobs.finish(job.ID, rep, runErr)
		if runErr == nil && req.Save {
			if id, saveErr := s.store.Save(rep); saveErr == nil {
				s.jobs.setStoreID(job.ID, id)
			}
		}
	}()

	writeJSON(w, http.StatusAccepted, jobView(job))
}

func (s *Server) handleListCrawls(w http.ResponseWriter, _ *http.Request) {
	jobs := s.jobs.list()
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].StartedAt.After(jobs[k].StartedAt) })

	saved := make(map[string]bool, len(jobs))
	items := make([]crawlListItem, 0, len(jobs))
	for _, j := range jobs {
		if j.StoreID != "" {
			saved[j.StoreID] = true
		}
		items = append(items, jobListItem(j))
	}

	entries, err := s.store.List("")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, e := range entries {
		if saved[e.ID] {
			continue
		}
		items = append(items, crawlListItem{
			ID:           e.ID,
			Seed:         e.Seed,
			Status:       string(StatusDone),
			FinishedAt:   e.FinishedAt,
			PagesCrawled: e.PagesCrawled,
			BySeverity:   e.BySeverity,
			Persisted:    true,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"crawls": items})
}

func (s *Server) handleGetCrawl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if job, ok := s.jobs.get(id); ok {
		writeJSON(w, http.StatusOK, jobView(job))
		return
	}
	rep, err := s.store.Load(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, JobView{
		ID:        id,
		Seed:      rep.Seed,
		Status:    string(StatusDone),
		Persisted: true,
		Report:    rep,
	})
}

func (s *Server) handleCancelCrawl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.jobs.cancel(id) {
		writeError(w, http.StatusNotFound, fmt.Errorf("no running crawl with id %q", id))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExportCrawl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var rep *report.Report
	if job, ok := s.jobs.get(id); ok {
		rep = job.Report
	}
	if rep == nil {
		loaded, err := s.store.Load(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		rep = loaded
	}
	if rep == nil {
		writeError(w, http.StatusConflict, errors.New("crawl has not finished yet"))
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	contentType, ext := exportContentType(format)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="gocrawl-report.%s"`, ext))
	if err := report.For(format).Write(w, rep); err != nil {
		writeError(w, http.StatusInternalServerError, err)
	}
}

func exportContentType(format string) (contentType, ext string) {
	switch format {
	case "csv":
		return "text/csv", "csv"
	case "html":
		return "text/html", "html"
	default:
		return "application/json", "json"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
