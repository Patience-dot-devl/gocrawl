package crawler

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

// Pause reasons reported in Progress.PauseReason.
const (
	PauseReasonUser        = "user"
	PauseReasonRateLimited = "rate_limited"
)

// errMaxDuration is the crawl context's cancellation cause when MaxDuration of active time
// runs out; Crawl uses it to tell the duration limit apart from an outside cancellation.
var errMaxDuration = errors.New("crawl reached its max duration")

// pauseState is the engine's pause bookkeeping, guarded by mu. mu sits between progMu and the
// leaf locks in the engine's lock order (see Engine): it may take frontier.mu, activeClock.mu
// and the limiter's locks, nothing holding those takes it, and it is always released before a
// progress event is emitted.
type pauseState struct {
	mu         sync.Mutex
	paused     bool
	reason     string
	retryAfter int
	fr         *frontier    // the running crawl's frontier; nil outside Crawl
	clock      *activeClock // the latest crawl's clock; kept after Crawl for ActiveTime
	running    bool         // Crawl is between beginRun and endRun
	finished   bool         // Crawl has returned; Pause and Resume do nothing until the next Crawl
}

// Pause stops handing out new fetches. Requests already in flight finish and are recorded.
// Called before Crawl, the crawl starts paused; called after Crawl returned, it does nothing.
// Cancelling the crawl's context still ends a paused crawl promptly, with a partial result.
// Pause must not be called from Options.OnProgress.
func (e *Engine) Pause() { e.pauseWith(PauseReasonUser, 0) }

// pauseWith pauses for reason and reports whether this call paused the crawl.
func (e *Engine) pauseWith(reason string, retryAfter int) bool {
	e.ps.mu.Lock()
	if e.ps.finished || e.ps.paused {
		e.ps.mu.Unlock()
		return false
	}
	e.ps.paused, e.ps.reason, e.ps.retryAfter = true, reason, retryAfter
	if e.ps.fr != nil {
		e.ps.fr.setPaused(true)
	}
	if e.ps.running {
		e.ps.clock.stop()
	}
	e.ps.mu.Unlock()
	e.logf("crawl paused (%s)", reason)
	e.emitState()
	return true
}

// Resume continues a paused crawl. rate > 0 replaces the request rate; 0 keeps it. It also
// clears the recent-429 window so auto-pause doesn't fire again on the old responses. It does
// nothing unless the crawl is paused (or queued to start paused). Resume must not be called
// from Options.OnProgress.
func (e *Engine) Resume(rate float64) {
	e.ps.mu.Lock()
	if e.ps.finished || !e.ps.paused {
		e.ps.mu.Unlock()
		return
	}
	e.ps.paused, e.ps.reason, e.ps.retryAfter = false, "", 0
	if rate > 0 {
		e.limiter.SetRate(rate)
	}
	e.limiter.ResetRecent()
	if e.ps.fr != nil {
		e.ps.fr.setPaused(false)
	}
	if e.ps.running {
		e.ps.clock.start()
	}
	e.ps.mu.Unlock()
	e.logf("crawl resumed")
	e.emitState()
}

// Paused reports whether the crawl is paused (or queued to start paused).
func (e *Engine) Paused() bool {
	e.ps.mu.Lock()
	defer e.ps.mu.Unlock()
	return e.ps.paused
}

// ActiveTime reports how long the current or last crawl has been running, excluding pauses.
// It is 0 before the first Crawl starts.
func (e *Engine) ActiveTime() time.Duration {
	e.ps.mu.Lock()
	c := e.ps.clock
	e.ps.mu.Unlock()
	if c == nil {
		return 0
	}
	return c.elapsed()
}

// beginRun attaches a starting crawl's frontier and clock, applying a pause queued earlier.
func (e *Engine) beginRun(fr *frontier, clock *activeClock) {
	e.ps.mu.Lock()
	defer e.ps.mu.Unlock()
	e.ps.fr, e.ps.clock, e.ps.running, e.ps.finished = fr, clock, true, false
	if e.ps.paused {
		fr.setPaused(true)
	} else {
		clock.start()
	}
}

// endRun detaches the finished crawl and reports whether it ended while paused (all queued
// work done or the context canceled), in which case the caller emits a final state event.
func (e *Engine) endRun() (wasPaused bool) {
	e.ps.mu.Lock()
	defer e.ps.mu.Unlock()
	wasPaused = e.ps.paused
	e.ps.clock.halt()
	e.ps.fr, e.ps.running, e.ps.finished = nil, false, true
	e.ps.paused, e.ps.reason, e.ps.retryAfter = false, "", 0
	return wasPaused
}

// Auto-pause tuning (Options.PauseOnThrottle).
const (
	autoPauseThreshold   = 10 // 429/503 responses within throttleWindow
	autoPauseMinBackoffs = 2  // adaptive-delay adjustments that must already have happened
)

// maybeAutoPause pauses the crawl when PauseOnThrottle is set and the server keeps answering
// 429/503. With adaptive delay on it waits until adaptive delay has backed off at least
// autoPauseMinBackoffs times; with it off nothing ever backs off, so the responses alone count.
// Called by a worker after each response; the worker holds no engine lock.
func (e *Engine) maybeAutoPause(status int) {
	if !e.opts.PauseOnThrottle {
		return
	}
	if status != http.StatusTooManyRequests && status != http.StatusServiceUnavailable {
		return
	}
	backoffs := e.limiter.ThrottleCount()
	if e.limiter.enabled && backoffs < autoPauseMinBackoffs {
		return
	}
	n := e.limiter.RecentThrottled()
	if n < autoPauseThreshold {
		return
	}
	if e.pauseWith(PauseReasonRateLimited, e.limiter.LastRetryAfter()) {
		e.logf("%d HTTP 429/503 responses in the last minute after %d backoffs — pausing the crawl", n, backoffs)
	}
}
