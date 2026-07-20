package webserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// Status is the lifecycle state of a crawl job.
type Status string

const (
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusError    Status = "error"
	StatusCanceled Status = "canceled"
)

// Job tracks one in-flight or just-finished crawl started via the web API.
type Job struct {
	ID         string
	Seed       string
	Status     Status
	Report     *report.Report
	Err        string
	StartedAt  time.Time
	FinishedAt time.Time
	StoreID    string // set once the report is saved, so history and job views agree on one ID

	cancel          context.CancelFunc
	cancelRequested bool
}

// jobManager holds every job the server has started, in memory, for the life of the process.
type jobManager struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func newJobManager() *jobManager {
	return &jobManager{jobs: make(map[string]*Job)}
}

func newJobID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// start, get, and list all return Job by value: a snapshot copied out while holding the lock.
// The canonical, mutable *Job lives only in m.jobs and is never handed to a caller, so a
// Report/Status read here can never race against finish/cancel mutating that same struct.
func (m *jobManager) start(seed string, cancel context.CancelFunc) Job {
	j := &Job{
		ID:        newJobID(),
		Seed:      seed,
		Status:    StatusRunning,
		StartedAt: time.Now(),
		cancel:    cancel,
	}
	m.mu.Lock()
	m.jobs[j.ID] = j
	snapshot := *j
	m.mu.Unlock()
	return snapshot
}

func (m *jobManager) get(id string) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	return *j, true
}

// list returns a snapshot of every job, in no particular order.
func (m *jobManager) list() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, *j)
	}
	return out
}

// finish records the outcome of a job. A canceled context does not make runner.Run return an
// error (see crawler.Coverage.Interrupted): it still produces a valid partial report, so the
// only way to tell a user-requested cancellation apart from a normal finish is the
// cancelRequested flag set by cancel below.
func (m *jobManager) finish(id string, rep *report.Report, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return
	}
	j.FinishedAt = time.Now()
	switch {
	case err != nil:
		j.Status = StatusError
		j.Err = err.Error()
	case j.cancelRequested:
		j.Status = StatusCanceled
		j.Report = rep
	default:
		j.Status = StatusDone
		j.Report = rep
	}
}

// cancel requests that a running job stop. It is a no-op if the job is already finished or
// unknown.
func (m *jobManager) cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || j.Status != StatusRunning {
		return false
	}
	j.cancelRequested = true
	j.cancel()
	return true
}

// setStoreID records that a job's report was persisted to the store under id, so the history
// listing doesn't show the same crawl twice.
func (m *jobManager) setStoreID(id, storeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[id]; ok {
		j.StoreID = storeID
	}
}

// JobView is the API representation of a Job.
type JobView struct {
	ID         string         `json:"id"`
	Seed       string         `json:"seed"`
	Status     string         `json:"status"`
	Err        string         `json:"error,omitempty"`
	StartedAt  string         `json:"started_at,omitempty"`
	FinishedAt string         `json:"finished_at,omitempty"`
	Persisted  bool           `json:"persisted"`
	Report     *report.Report `json:"report,omitempty"`
}

func jobView(j Job) JobView {
	return JobView{
		ID:         j.ID,
		Seed:       j.Seed,
		Status:     string(j.Status),
		Err:        j.Err,
		StartedAt:  formatTime(j.StartedAt),
		FinishedAt: formatTime(j.FinishedAt),
		Persisted:  j.StoreID != "",
		Report:     j.Report,
	}
}

// crawlListItem is the lightweight API representation used by the crawl list endpoint —
// summary counts only, not the full report.
type crawlListItem struct {
	ID           string         `json:"id"`
	Seed         string         `json:"seed"`
	Status       string         `json:"status"`
	Err          string         `json:"error,omitempty"`
	StartedAt    string         `json:"started_at,omitempty"`
	FinishedAt   string         `json:"finished_at,omitempty"`
	PagesCrawled int            `json:"pages_crawled,omitempty"`
	BySeverity   map[string]int `json:"by_severity,omitempty"`
	Persisted    bool           `json:"persisted"`
}

func jobListItem(j Job) crawlListItem {
	item := crawlListItem{
		ID:         j.ID,
		Seed:       j.Seed,
		Status:     string(j.Status),
		Err:        j.Err,
		StartedAt:  formatTime(j.StartedAt),
		FinishedAt: formatTime(j.FinishedAt),
		Persisted:  j.StoreID != "",
	}
	if j.Report != nil {
		item.PagesCrawled = j.Report.PagesCrawled
		item.BySeverity = j.Report.Summary.BySeverity
	}
	return item
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
