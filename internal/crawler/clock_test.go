package crawler

import (
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
	c := newActiveClock(0, nil)
	c.halt()
	c.start()
	time.Sleep(20 * time.Millisecond)
	if got := c.elapsed(); got != 0 {
		t.Errorf("elapsed after halt then start = %v, want 0", got)
	}
}
