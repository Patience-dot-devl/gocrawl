package webserver

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

// progressView is JobView.progress: the latest runner.Progress plus the tape from tape_from.
type progressView struct {
	runner.Progress
	Tape     string `json:"tape"`
	TapeOff  string `json:"tape_off"`
	TapeFrom int    `json:"tape_from"` // index the returned tape slice starts at
	MaxPages int    `json:"max_pages"`
	ActiveMS int64  `json:"active_ms"`
}

// crawlRunOptions are the runner options every web crawl gets. Someone can press Resume in the
// UI, so a crawl the server keeps rate limiting pauses itself instead of grinding on.
func (s *Server) crawlRunOptions(id string, ctl *runner.Control) []runner.RunOption {
	return []runner.RunOption{
		runner.WithProgress(func(p runner.Progress) { s.recordProgress(id, p) }),
		runner.WithControl(ctl),
		runner.WithPauseOnThrottle(),
	}
}

// recordProgress stores the latest snapshot and grows the tape by one character per recorded
// page, with the same helpers runner.Run uses for the saved Coverage.Tape (crawler.TapeChar,
// Progress.TapeOffChar), so the live and saved tapes match. It runs on a crawl worker goroutine
// (engine locks held), so it does nothing but a few assignments under the job manager's lock.
func (s *Server) recordProgress(id string, p runner.Progress) {
	s.crawls.with(id, func(j *Job[crawlJob]) {
		snap := p
		j.Data.Progress = &snap
		if p.Fetched > len(j.Data.Tape) {
			j.Data.Tape = append(j.Data.Tape, crawler.TapeChar(p.Status))
			j.Data.TapeOff = append(j.Data.TapeOff, p.TapeOffChar())
		}
		// Follow the engine's pause state, so an automatic pause (and the resume after it)
		// shows in the job status. Only running/paused jobs move. A paused job only goes back
		// to running on a crawling-phase event: during the sitemap phase a queued pause has not
		// reached the engine yet, and its events still say "not paused".
		switch {
		case p.Paused && j.Status == StatusRunning:
			j.Status = StatusPaused
		case !p.Paused && j.Status == StatusPaused && p.Phase == runner.PhaseCrawling:
			j.Status = StatusRunning
		}
	})
}

// newProgressView builds the API view of j's progress. Call it inside s.crawls.with: it reads
// the tape recordProgress appends to.
func newProgressView(j *Job[crawlJob], tapeFrom int, now time.Time) *progressView {
	if j.Data.Progress == nil {
		return nil
	}
	if tapeFrom > len(j.Data.Tape) {
		tapeFrom = len(j.Data.Tape)
	}
	return &progressView{
		Progress: *j.Data.Progress,
		Tape:     string(j.Data.Tape[tapeFrom:]),
		TapeOff:  string(j.Data.TapeOff[tapeFrom:]),
		TapeFrom: tapeFrom,
		MaxPages: j.Data.MaxPages,
		ActiveMS: activeMS(j, now),
	}
}

// activeMS is the crawl phase's active time (pauses excluded). Control.Active is lock-free,
// so this is safe inside s.crawls.with.
func activeMS(j *Job[crawlJob], now time.Time) int64 {
	if j.Data.control != nil {
		return j.Data.control.Active().Milliseconds()
	}
	end := now
	if !j.FinishedAt.IsZero() {
		end = j.FinishedAt
	}
	return end.Sub(j.StartedAt).Milliseconds()
}

// crawlView is the JobView for a live or finished in-memory job, with the tape sliced from
// tapeFrom. False when id is not an in-memory job.
func (s *Server) crawlView(id string, tapeFrom int) (JobView, bool) {
	var v JobView
	ok := s.crawls.with(id, func(j *Job[crawlJob]) {
		v = jobView(*j)
		v.Progress = newProgressView(j, tapeFrom, time.Now())
	})
	return v, ok
}

// parseTapeFrom reads the tape_from query parameter (default 0).
func parseTapeFrom(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("tape_from must be a non-negative integer, got %q", raw)
	}
	return n, nil
}

// classesFromByStatus folds a report's pages_by_status into status classes, for reports saved
// before tapes were stored.
func classesFromByStatus(byStatus map[string]int) map[string]int {
	if len(byStatus) == 0 {
		return nil
	}
	out := map[string]int{}
	for code, n := range byStatus {
		status, _ := strconv.Atoi(code)
		out[string(crawler.TapeChar(status))] += n
	}
	return out
}

// savedClasses is a stored crawl's status classes: counted from its tape when it has one, so a
// crawl shows the same classes live and after a restart, else folded from pages_by_status.
func savedClasses(e store.Entry) map[string]int {
	if len(e.StatusClasses) > 0 {
		return e.StatusClasses
	}
	return classesFromByStatus(e.ByStatus)
}

// listProgress returns the status classes, sitemap coverage and tape preview for a crawl list
// row. j is a list() snapshot: reading j.Data.Tape[:len] outside the lock is safe because later
// appends only write past that length.
func listProgress(j Job[crawlJob]) (classes map[string]int, sitemapTotal, sitemapFetched int, preview string) {
	rep := j.Data.Report
	switch {
	case len(j.Data.Tape) > 0:
		tape := string(j.Data.Tape)
		classes = crawler.TapeClasses(tape)
		preview = crawler.TapePreview(tape, crawler.TapePreviewLen)
	case rep != nil:
		if rep.Coverage != nil && rep.Coverage.Tape != "" {
			classes = crawler.TapeClasses(rep.Coverage.Tape)
			preview = crawler.TapePreview(rep.Coverage.Tape, crawler.TapePreviewLen)
		} else {
			classes = classesFromByStatus(rep.Summary.ByStatus)
		}
	}
	if rep != nil && rep.Coverage != nil {
		return classes, rep.Coverage.SitemapTotal, rep.Coverage.SitemapFetched, preview
	}
	if p := j.Data.Progress; p != nil {
		return classes, p.SitemapTotal, p.SitemapFetched, preview
	}
	return classes, 0, 0, preview
}
