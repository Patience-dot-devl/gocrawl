package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// Bounds on the resume rate (requests per second): a typo like 1e9 must not turn a throttled
// crawl into a flood, and a rate below the adaptive floor is almost certainly a mistake.
const (
	minResumeRate = 0.001
	maxResumeRate = 1000

	maxResumeBody = 4 << 10
)

// resumeRequest is the POST /api/crawls/{id}/resume body. Rate, when set, is the new
// requests-per-second limit; omitted keeps the current one.
type resumeRequest struct {
	Rate *float64 `json:"rate,omitempty"`
}

func (s *Server) handlePauseCrawl(w http.ResponseWriter, r *http.Request) {
	job, ok := s.crawls.get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errCrawlNotFound)
		return
	}
	if job.Status != StatusRunning || job.Data.control == nil {
		writeError(w, http.StatusConflict, fmt.Errorf("the crawl is %s, not running, so it can't be paused", job.Status))
		return
	}
	// Outside s.crawls.with: Pause emits a progress event, and the progress callback takes the
	// job manager's lock itself.
	if !job.Data.control.Pause() {
		writeError(w, http.StatusConflict, errors.New("the crawl phase is over; there is nothing left to pause"))
		return
	}
	// A pause queued during the sitemap phase emits nothing yet, so record it here. Once the
	// engine is attached its own events set the status; writing it here as well would race
	// them (a pause that lands as the last in-flight page ends would stay "paused" through
	// the analysis phase).
	s.crawls.with(job.ID, func(j *Job[crawlJob]) {
		p := j.Data.Progress
		if p != nil && p.Phase != runner.PhaseSitemap {
			return
		}
		if j.Status == StatusRunning {
			j.Status = StatusPaused
		}
		if p != nil && !p.Paused {
			cp := *p
			cp.Paused, cp.PauseReason = true, crawler.PauseReasonUser
			j.Data.Progress = &cp
		}
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleResumeCrawl(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var req resumeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxResumeBody)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("request body is larger than %d bytes", maxResumeBody))
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	rate := 0.0
	if req.Rate != nil {
		if *req.Rate < minResumeRate || *req.Rate > maxResumeRate {
			writeError(w, http.StatusBadRequest, fmt.Errorf("rate must be between %g and %g requests per second", minResumeRate, float64(maxResumeRate)))
			return
		}
		rate = *req.Rate
	}

	job, ok := s.crawls.get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errCrawlNotFound)
		return
	}
	if job.Status != StatusPaused || job.Data.control == nil {
		writeError(w, http.StatusConflict, fmt.Errorf("the crawl is %s, not paused, so it can't be resumed", job.Status))
		return
	}
	// Outside s.crawls.with, for the same reason as in handlePauseCrawl.
	if !job.Data.control.Resume(rate) {
		writeError(w, http.StatusConflict, errors.New("the crawl phase is over; there is nothing left to resume"))
		return
	}
	// Same rule as in handlePauseCrawl: only a queued pause (sitemap phase) is recorded here.
	s.crawls.with(job.ID, func(j *Job[crawlJob]) {
		if p := j.Data.Progress; p != nil && p.Phase != runner.PhaseSitemap {
			return
		}
		if j.Status == StatusPaused {
			j.Status = StatusRunning
		}
		if p := j.Data.Progress; p != nil && p.Paused {
			cp := *p
			cp.Paused, cp.PauseReason, cp.RetryAfterSeconds = false, "", 0
			if rate > 0 {
				cp.Rate = rate
			}
			j.Data.Progress = &cp
		}
	})
	w.WriteHeader(http.StatusNoContent)
}
