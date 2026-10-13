package webserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Status is the lifecycle state of a background job: a crawl or a redirect check.
type Status string

const (
	StatusRunning  Status = "running"
	StatusPaused   Status = "paused"
	StatusDone     Status = "done"
	StatusError    Status = "error"
	StatusCanceled Status = "canceled"
)

// Job is one background job started through the web API. The lifecycle fields are shared by
// every kind of job; Data holds what is specific to one kind (crawlJob, redirectJob).
type Job[T any] struct {
	ID         string
	Status     Status
	Err        string
	StartedAt  time.Time
	FinishedAt time.Time
	Data       T

	cancel          context.CancelFunc
	cancelRequested bool
}

// active reports whether the job still occupies a slot under its manager's concurrency cap.
// A paused job holds its slot: it has an engine, a fetcher and a frontier in memory, and it
// can resume at any moment.
func (j *Job[T]) active() bool {
	return j.Status == StatusRunning || j.Status == StatusPaused
}

// jobManager holds every job of one kind that the server has started, in memory, for the
// life of the process. Crawls and redirect checks each get their own manager, and so their
// own cap.
type jobManager[T any] struct {
	mu   sync.Mutex
	jobs map[string]*Job[T]
}

func newJobManager[T any]() *jobManager[T] {
	return &jobManager[T]{jobs: make(map[string]*Job[T])}
}

func newJobID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// start, get, and list all return Job by value: a snapshot copied out while holding the lock.
// The canonical, mutable *Job lives only in m.jobs and is never handed to a caller except
// inside with, so a read of a snapshot can never race against finish/cancel/with mutating
// that same struct. The copy is shallow: slices and pointers in Data still share their
// backing memory, so code that reads something another goroutine appends to (the crawl
// tape) reads it inside with instead.
//
// start refuses (ok == false) when maxRunning jobs are already running or paused, so the
// count and the insert happen under one lock and two concurrent starts cannot both slip
// past the cap.
func (m *jobManager[T]) start(data T, cancel context.CancelFunc, maxRunning int) (Job[T], bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	active := 0
	for _, j := range m.jobs {
		if j.active() {
			active++
		}
	}
	if active >= maxRunning {
		return Job[T]{}, false
	}
	j := &Job[T]{
		ID:        newJobID(),
		Status:    StatusRunning,
		StartedAt: time.Now(),
		Data:      data,
		cancel:    cancel,
	}
	m.jobs[j.ID] = j
	return *j, true
}

func (m *jobManager[T]) get(id string) (Job[T], bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return Job[T]{}, false
	}
	return *j, true
}

// list returns a snapshot of every job, in no particular order.
func (m *jobManager[T]) list() []Job[T] {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Job[T], 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, *j)
	}
	return out
}

// with runs fn on the live job under the manager's lock and reports whether the job exists.
// Use it to update Data while the job runs (progress, tape) and to build views that must
// not race with those updates. fn must not block and must not call back into m.
func (m *jobManager[T]) with(id string, fn func(j *Job[T])) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return false
	}
	fn(j)
	return true
}

// finish records the outcome of a job: the finish time, the final status, and the error
// text. apply, when non-nil, runs under the lock on the job's Data, whatever the outcome, so
// the result lands in the same critical section as the status change and a poller never
// sees "done" without it.
//
// A canceled context does not make runner.Run return an error (see
// crawler.Coverage.Interrupted): it still produces a valid partial report, so the only way
// to tell a user-requested cancellation apart from a normal finish is the cancelRequested
// flag set by cancel below.
func (m *jobManager[T]) finish(id string, err error, apply func(d *T)) {
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
	default:
		j.Status = StatusDone
	}
	if apply != nil {
		apply(&j.Data)
	}
}

// cancel requests that a running or paused job stop. It is a no-op (false) if the job is
// already finished or unknown.
func (m *jobManager[T]) cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || !j.active() {
		return false
	}
	j.cancelRequested = true
	j.cancel()
	return true
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
