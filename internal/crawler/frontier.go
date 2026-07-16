package crawler

import (
	"context"
	"sync"
)

// frontier is a FIFO work queue shared by a fixed pool of workers, sized to the discovered
// site rather than to concurrency: pushing a task never blocks, so the number of goroutines in
// flight is bounded by the worker count instead of growing with the frontier (as a
// goroutine-per-URL crawler's does). Pulling in FIFO order also gives the crawl a true
// breadth-first traversal, which — unlike the previous approach of racing a fresh goroutine per
// link against a semaphore — is a natural place to add future per-host politeness (e.g. an
// enforced Crawl-delay) without disturbing overall ordering.
//
// Termination uses reference counting: pending counts tasks that are queued or being
// processed by a worker. push increments it; taskDone (called exactly once per task returned
// by next) decrements it. The frontier is exhausted once pending reaches zero with the queue
// empty — by construction this can't happen while a task that might still push children is
// outstanding, since a worker always pushes a task's children before calling taskDone for it.
type frontier struct {
	mu      sync.Mutex
	cond    *sync.Cond
	queue   []task
	pending int
}

func newFrontier() *frontier {
	f := &frontier{}
	f.cond = sync.NewCond(&f.mu)
	return f
}

// push adds t to the queue and marks it pending.
func (f *frontier) push(t task) {
	f.mu.Lock()
	f.pending++
	f.queue = append(f.queue, t)
	f.mu.Unlock()
	f.cond.Signal()
}

// next blocks until a task is available, ctx is done, or the frontier is exhausted (nothing
// queued and nothing pending), returning ok=false in the latter two cases. Checking ctx first
// on every wake means a cancellation abandons any remaining queued work immediately rather
// than draining it.
func (f *frontier) next(ctx context.Context) (task, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for {
		if ctx.Err() != nil {
			return task{}, false
		}
		if len(f.queue) > 0 {
			t := f.queue[0]
			f.queue = f.queue[1:]
			return t, true
		}
		if f.pending == 0 {
			return task{}, false
		}
		f.cond.Wait()
	}
}

// taskDone marks one task, previously returned by next, as fully processed — including having
// already pushed any children it discovered. Must be called exactly once per task from next.
func (f *frontier) taskDone() {
	f.mu.Lock()
	f.pending--
	f.mu.Unlock()
	// Broadcast unconditionally, not just at pending == 0: workers blocked in next() only
	// re-check their wait condition (including ctx.Err()) when woken, so a cancellation needs
	// a wake-up from somewhere to be noticed promptly rather than sitting until the next
	// unrelated push/taskDone.
	f.cond.Broadcast()
}

// watchCancellation wakes every worker blocked in next() as soon as ctx is done, so a
// cancellation is noticed promptly instead of only on the next push/taskDone. The returned
// stop func must be called once ctx can no longer fire (i.e. once the crawl using this
// frontier has finished) to release the goroutine.
func (f *frontier) watchCancellation(ctx context.Context) (stop func()) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			f.mu.Lock()
			f.cond.Broadcast()
			f.mu.Unlock()
		case <-done:
		}
	}()
	return func() { close(done) }
}
