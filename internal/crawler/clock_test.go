package crawler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestActiveClockExcludesStoppedTime(t *testing.T) {
	c := newActiveClock(0, nil)
	c.start()
	time.Sleep(20 * time.Millisecond)
	c.stop()
	frozen := c.elapsed()
	time.Sleep(400 * time.Millisecond)
	if got := c.elapsed(); got != frozen {
		t.Fatalf("elapsed moved from %v to %v while stopped", frozen, got)
	}
	c.start()
	time.Sleep(20 * time.Millisecond)
	c.halt()
	got := c.elapsed()
	if got < 40*time.Millisecond {
		t.Errorf("elapsed = %v, want at least the 40ms of running time", got)
	}
	if got >= 400*time.Millisecond {
		t.Errorf("elapsed = %v, want the 400ms stop excluded", got)
	}
}

func TestActiveClockFiresOnlyOnActiveTime(t *testing.T) {
	fired := make(chan struct{})
	c := newActiveClock(200*time.Millisecond, func() { close(fired) })
	c.start()
	time.Sleep(20 * time.Millisecond)
	c.stop()
	select {
	case <-fired:
		t.Fatal("a stopped clock fired")
	case <-time.After(400 * time.Millisecond):
	}
	c.start()
	select {
	case <-fired:
	case <-time.After(3 * time.Second):
		t.Fatal("clock did not fire once the remaining active time ran out")
	}
	c.halt()
}

func TestActiveClockHaltIsFinal(t *testing.T) {
	c := newActiveClock(10*time.Millisecond, func() { t.Error("a halted clock fired") })
	c.halt()
	c.start()
	time.Sleep(100 * time.Millisecond) // well past the limit
	if got := c.elapsed(); got != 0 {
		t.Errorf("elapsed after halt then start = %v, want 0", got)
	}
}

func TestActiveClockFiresAtMostOnce(t *testing.T) {
	var fired atomic.Int32
	c := newActiveClock(10*time.Millisecond, func() { fired.Add(1) })
	c.start()
	deadline := time.Now().Add(3 * time.Second)
	for fired.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if fired.Load() == 0 {
		t.Fatal("clock did not fire")
	}
	// Restarting with the budget already used up must not fire again.
	c.stop()
	c.start()
	time.Sleep(100 * time.Millisecond)
	c.halt()
	if got := fired.Load(); got != 1 {
		t.Errorf("onLimit called %d times, want exactly once", got)
	}
}
