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
)

// crawlRoutes registers the crawl job API plus the analyzer and explanation listings the
// New crawl form and the report view read.
func (s *Server) crawlRoutes() {
	s.mux.HandleFunc("GET /api/analyzers", s.handleListAnalyzers)
	s.mux.HandleFunc("GET /api/explanations", s.handleListExplanations)
	s.mux.HandleFunc("POST /api/crawls", s.handleStartCrawl)
	s.mux.HandleFunc("GET /api/crawls", s.handleListCrawls)
	s.mux.HandleFunc("GET /api/crawls/{id}", s.handleGetCrawl)
	s.mux.HandleFunc("POST /api/crawls/{id}/cancel", s.handleCancelCrawl)
	s.mux.HandleFunc("GET /api/crawls/{id}/export", s.handleExportCrawl)
}

func (s *Server) handleListAnalyzers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"analyzers": runner.ListAnalyzers()})
}

// handleListExplanations returns the same code -> what/impact/fix text the HTML report bakes
// in per issue, so the live report view can render identical explanations without duplicating
// internal/report/explanations.go in the frontend.
func (s *Server) handleListExplanations(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"explanations": report.Explanations()})
}

// startCrawlRequest is the POST /api/crawls body: the shared crawl params plus web-only
// options.
type startCrawlRequest struct {
	crawlrequest.Params
	Save bool `json:"save"`
}

func (s *Server) handleStartCrawl(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
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
	job, ok := s.crawls.start(crawlJob{Seed: seed, MaxPages: cfg.Crawl.MaxPages}, cancel, s.maxRunning)
	if !ok {
		cancel()
		writeError(w, http.StatusTooManyRequests, fmt.Errorf("%d crawls already running; wait for one to finish or cancel it", s.maxRunning))
		return
	}

	go func() {
		defer cancel()
		rep, runErr := runner.Run(ctx, cfg, seed, s.crawlRunOptions(job.ID)...)
		// Save before finishing, so a poller that sees the job finish also sees it persisted.
		var storeID string
		if runErr == nil && req.Save {
			if id, saveErr := s.store.Save(rep); saveErr == nil {
				storeID = id
			}
		}
		s.crawls.finish(job.ID, runErr, func(d *crawlJob) {
			d.Report = rep
			d.StoreID = storeID
		})
	}()

	writeJSON(w, http.StatusAccepted, jobView(job))
}

func (s *Server) handleListCrawls(w http.ResponseWriter, _ *http.Request) {
	jobs := s.crawls.list()
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].StartedAt.After(jobs[k].StartedAt) })

	saved := make(map[string]bool, len(jobs))
	items := make([]crawlListItem, 0, len(jobs))
	for _, j := range jobs {
		if j.Data.StoreID != "" {
			saved[j.Data.StoreID] = true
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
			ID:             e.ID,
			Seed:           e.Seed,
			Status:         string(StatusDone),
			FinishedAt:     e.FinishedAt,
			PagesCrawled:   e.PagesCrawled,
			BySeverity:     e.BySeverity,
			Persisted:      true,
			StatusClasses:  savedClasses(e),
			SitemapTotal:   e.SitemapTotal,
			SitemapFetched: e.SitemapFetched,
			TapePreview:    e.TapePreview,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"crawls": items})
}

func (s *Server) handleGetCrawl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tapeFrom, err := parseTapeFrom(r.URL.Query().Get("tape_from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if view, ok := s.crawlView(id, tapeFrom); ok {
		writeJSON(w, http.StatusOK, view)
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
	if !s.crawls.cancel(id) {
		writeError(w, http.StatusNotFound, fmt.Errorf("no running crawl with id %q", id))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExportCrawl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var rep *report.Report
	if job, ok := s.crawls.get(id); ok {
		rep = job.Data.Report
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
