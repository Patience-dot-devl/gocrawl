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
// link against a semaphore — lets a worker wait out a host's robots.txt Crawl-delay
// (Engine.waitCrawlDelay) without disturbing overall ordering. While a worker waits on a host,
// next skips that host's tasks (see busy), so the other workers keep crawling other hosts.
//
// Termination uses reference counting: pending counts tasks that are queued or being
// processed by a worker. push and pushFront increment it; taskDone (called exactly once per task returned
// by next) decrements it. The frontier is exhausted once pending reaches zero with the queue
// empty — by construction this can't happen while a task that might still push children is
// outstanding, since a worker always pushes a task's children before calling taskDone for it.
type frontier struct {
	mu      sync.Mutex
	cond    *sync.Cond
	queue   []task
	pending int
	// paused makes next hold back queued tasks (Engine.Pause). Cancellation and exhaustion
	// still end next, so a paused crawl can be stopped and a crawl with nothing left ends.
	paused bool
	// busy, when set, reports a host that a worker is already waiting on (its robots.txt
	// Crawl-delay; see hostPacer.busy). next skips that host's tasks and hands out the first
	// task for another host instead, so one slow host cannot hold every worker while other
	// hosts' tasks wait in the queue. It is called with mu held: frontier.mu → hostPacer.mu.
	busy func(host string) bool
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

// queued reports how many tasks are waiting to be handed to a worker.
func (f *frontier) queued() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.queue)
}

// next blocks until a task is available, ctx is done, or the frontier is exhausted (nothing
// queued and nothing pending), returning ok=false in the latter two cases. While paused it
// hands out nothing but keeps waiting. Checking ctx first on every wake means a cancellation
// abandons any remaining queued work immediately rather than draining it.
func (f *frontier) next(ctx context.Context) (task, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for {
		if ctx.Err() != nil {
			return task{}, false
		}
		// pending counts queued tasks too, so pending == 0 means the queue is empty.
		if f.pending == 0 {
			return task{}, false
		}
		if !f.paused {
			if i := f.firstReady(); i >= 0 {
				t := f.queue[i]
				if i == 0 {
					f.queue = f.queue[1:]
				} else {
					f.queue = append(f.queue[:i], f.queue[i+1:]...)
				}
				return t, true
			}
		}
		f.cond.Wait()
	}
}

// firstReady returns the index of the first queued task whose host no worker is waiting on,
// or -1 when there is none. Without a busy func, or when the head's host is free (always, on
// a crawl with no Crawl-delay), it is the head: plain FIFO order. Must be called with mu held.
func (f *frontier) firstReady() int {
	for i, t := range f.queue {
		if f.busy == nil || t.host == "" || !f.busy(t.host) {
			return i
		}
	}
	return -1
}

// wake makes every worker blocked in next look at the queue again, for example after a
// host stops being busy.
func (f *frontier) wake() {
	f.mu.Lock()
	f.cond.Broadcast()
	f.mu.Unlock()
}

// setPaused holds back (true) or releases (false) queued tasks and wakes every waiter.
func (f *frontier) setPaused(p bool) {
	f.mu.Lock()
	f.paused = p
	f.mu.Unlock()
	f.cond.Broadcast()
}

// pushFront puts a task a worker took but did not process back at the head of the queue and
// marks it pending again. The worker must still call taskDone for its original turn; pending
// never touches zero in between because pushFront increments first.
func (f *frontier) pushFront(t task) {
	f.mu.Lock()
	f.pending++
	f.queue = append([]task{t}, f.queue...)
	f.mu.Unlock()
	f.cond.Signal()
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
