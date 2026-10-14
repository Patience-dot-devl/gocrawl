package runner

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Control pauses and resumes the crawl inside a Run started with WithControl. Run hides the
// engine, so Control forwards to it while the crawl phase runs. Calls before the crawl phase
// are queued (a pause made during the sitemap pre-pass takes effect when crawling starts);
// calls after it ends do nothing and return false.
//
// Lock order: Control.mu is taken before the engine's locks (Pause and Resume emit a progress
// event while it is held). Nothing on the progress path takes Control.mu. Active takes no
// Control lock, but through Engine.ActiveTime it briefly takes the engine's ps.mu and then
// the activeClock's mutex, one after the other. A caller may therefore call Active while
// holding its own job lock (that adds a job lock → ps.mu edge, noted in the Engine lock
// order), but must not call Pause or Resume while holding a lock its progress callback takes.
type Control struct {
	mu      sync.Mutex
	engine  *crawler.Engine // set during the crawl phase
	pending bool            // pause requested before the crawl phase began
	// pendingRate is a Resume rate given before the crawl phase began; attach applies it.
	pendingRate float64
	ended       bool

	live   atomic.Pointer[crawler.Engine] // mirrors engine for the lock-free Active
	frozen atomic.Int64                   // active time in ns, fixed when the crawl phase ends
}

// NewControl returns a Control to pass to WithControl.
func NewControl() *Control { return &Control{} }

// Pause pauses the crawl, or queues the pause if crawling hasn't started. It returns false
// once the crawl phase is over.
func (c *Control) Pause() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended {
		return false
	}
	if c.engine == nil {
		c.pending = true
		return true
	}
	c.engine.Pause()
	return c.engine.Paused()
}

// Resume resumes a paused crawl at rate requests per second (0 keeps the current rate), or
// cancels a queued pause (a rate given then is kept and applied when crawling starts). It returns false once the crawl phase is over.
func (c *Control) Resume(rate float64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended {
		return false
	}
	if c.engine == nil {
		c.pending = false
		if rate > 0 {
			c.pendingRate = rate
		}
		return true
	}
	c.engine.Resume(rate)
	return true
}

// Active reports the crawl's active time (pauses excluded): 0 before the crawl phase, live
// during it, frozen after it. Safe to call while holding any lock.
func (c *Control) Active() time.Duration {
	if e := c.live.Load(); e != nil {
		return e.ActiveTime()
	}
	return time.Duration(c.frozen.Load())
}

// attach hands the engine over right before Engine.Crawl, applying a queued pause.
func (c *Control) attach(e *crawler.Engine) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.engine = e
	c.live.Store(e)
	if c.pendingRate > 0 {
		e.SetRate(c.pendingRate)
		c.pendingRate = 0
	}
	if c.pending {
		c.pending = false
		e.Pause()
	}
}

// detach ends the crawl phase right after Engine.Crawl returns.
func (c *Control) detach() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.engine != nil {
		c.frozen.Store(int64(c.engine.ActiveTime()))
	}
	c.live.Store(nil)
	c.engine = nil
	c.ended = true
}
