package crawler

import (
	"sync"
	"time"
)

// activeClock measures a crawl's active time (wall-clock time minus time spent paused) and
// calls onLimit once that reaches limit (0 = no limit). It is the replacement for the
// context.WithTimeout MaxDuration used to be, which kept counting while a crawl sat paused.
//
// onLimit is called at most once over the clock's life, and no call starts after halt. Lock
// order: activeClock.mu is a leaf lock. Nothing is called while it is held, and onLimit runs
// on a timer goroutine without it, so onLimit may take any other lock (Engine passes a context
// cancel). stop disarms the timer, but a timer that expired just before stop may still call
// onLimit, so onLimit must tolerate firing just after stop.
type activeClock struct {
	limit   time.Duration
	onLimit func()

	mu     sync.Mutex
	acc    time.Duration // active time banked by earlier runs
	since  time.Time     // start of the current run; zero while stopped
	timer  *time.Timer
	halted bool
	fired  bool // onLimit has been called (or is being called)
}

func newActiveClock(limit time.Duration, onLimit func()) *activeClock {
	return &activeClock{limit: limit, onLimit: onLimit}
}

// start begins (or resumes) counting. A no-op while running or after halt.
func (c *activeClock) start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.halted || !c.since.IsZero() {
		return
	}
	c.since = time.Now()
	if c.limit > 0 && c.onLimit != nil {
		remaining := max(c.limit-c.acc, 0)
		c.timer = time.AfterFunc(remaining, c.fire)
	}
}

// fire calls onLimit unless it already ran or the clock was halted. A restart with the budget
// already used up arms a zero timer, so without the fired flag onLimit would run again.
func (c *activeClock) fire() {
	c.mu.Lock()
	if c.fired || c.halted {
		c.mu.Unlock()
		return
	}
	c.fired = true
	c.mu.Unlock()
	c.onLimit()
}

// stop pauses counting and disarms the limit timer.
func (c *activeClock) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

func (c *activeClock) stopLocked() {
	if c.since.IsZero() {
		return
	}
	c.acc += time.Since(c.since)
	c.since = time.Time{}
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
}

// halt stops the clock for good; elapsed keeps returning the final value.
func (c *activeClock) halt() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
	c.halted = true
}

// elapsed reports the active time so far.
func (c *activeClock) elapsed() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	d := c.acc
	if !c.since.IsZero() {
		d += time.Since(c.since)
	}
	return d
}
