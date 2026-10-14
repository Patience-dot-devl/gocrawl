package runner

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

func newEngine() *crawler.Engine {
	opts := crawler.DefaultOptions()
	return crawler.New(opts, crawler.NewHTTPFetcher(opts))
}

func TestControlQueuesPauseBeforeTheCrawlPhase(t *testing.T) {
	c := NewControl()
	if !c.Pause() {
		t.Fatal("Pause before the crawl phase should be queued and return true")
	}
	e := newEngine()
	c.attach(e)
	if !e.Paused() {
		t.Fatal("a queued pause must reach the engine when the crawl phase starts")
	}
	if !c.Resume(0) || e.Paused() {
		t.Fatal("Resume during the crawl phase should resume the engine")
	}
	c.detach()
	if c.Pause() || c.Resume(0) {
		t.Error("Pause/Resume after the crawl phase must return false")
	}
}

func TestControlResumeBeforeCrawlCancelsQueuedPause(t *testing.T) {
	c := NewControl()
	c.Pause()
	c.Resume(0)
	e := newEngine()
	c.attach(e)
	if e.Paused() {
		t.Error("a pause cancelled before the crawl started must not reach the engine")
	}
	c.detach()
}

func TestControlNilIsSafe(t *testing.T) {
	var c *Control
	c.attach(newEngine())
	c.detach()
}

func TestWithPauseOnThrottleSetsTheSetting(t *testing.T) {
	var s runSettings
	WithPauseOnThrottle()(&s)
	if !s.pauseOnThrottle {
		t.Error("WithPauseOnThrottle did not turn the setting on")
	}
}

func TestRunWithControlPausesAndResumes(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	ctl := NewControl()
	ctl.Pause() // queued: the crawl phase starts paused

	var rep *report.Report
	done := make(chan error, 1)
	go func() {
		r, err := Run(context.Background(), config.Default(), ts.URL, WithControl(ctl))
		rep = r
		done <- err
	}()
	time.Sleep(300 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("Run finished while its crawl was paused")
	default:
	}
	if !ctl.Resume(0) {
		t.Fatal("Resume during the crawl phase returned false")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not finish after Resume")
	}
	if rep.PagesCrawled != 4 {
		t.Errorf("PagesCrawled = %d, want 4", rep.PagesCrawled)
	}
	if ctl.Pause() {
		t.Error("Pause after Run returned should be false")
	}
	if ctl.Active() <= 0 || ctl.Active() >= 300*time.Millisecond {
		t.Errorf("Active = %v, want the crawl's active time without the 300ms pause", ctl.Active())
	}
}

// A pause queued during the sitemap phase, then a cancel while the crawl still sits paused (the
// web UI's Pause followed by Stop crawl before anything was fetched): Run must return its
// partial report promptly instead of waiting for a Resume that never comes.
func TestRunCanceledWhileQueuedPauseHolds(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	ctl := NewControl()
	ctl.Pause()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, config.Default(), ts.URL, WithControl(ctl))
		done <- err
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v (a canceled crawl returns a partial report, not an error)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel while its crawl was paused")
	}
	if ctl.Pause() || ctl.Resume(0) {
		t.Error("Pause/Resume after Run returned must be false")
	}
}

func TestControlResumeRateBeforeAttachReachesTheEngine(t *testing.T) {
	c := NewControl()
	c.Pause()
	c.Resume(2.5) // cancels the queued pause but keeps the chosen rate
	e := newEngine()
	c.attach(e)
	defer c.detach()
	if e.Paused() {
		t.Error("the cancelled pause reached the engine")
	}
	if got := e.EffectiveRate(); got != 2.5 {
		t.Errorf("EffectiveRate = %v, want 2.5", got)
	}

	// A later Resume(0) must not discard a pending rate before attach, and a rate of 0 alone
	// leaves the engine's configured rate alone.
	c = NewControl()
	c.Resume(0)
	e = newEngine()
	want := e.EffectiveRate()
	c.attach(e)
	defer c.detach()
	if got := e.EffectiveRate(); got != want {
		t.Errorf("EffectiveRate = %v, want the configured %v", got, want)
	}
}
