package crawler

import (
	"context"
	"testing"
	"time"
)

func TestFrontierFIFOOrder(t *testing.T) {
	f := newFrontier()
	f.push(task{url: "a"})
	f.push(task{url: "b"})
	f.push(task{url: "c"})

	for _, want := range []string{"a", "b", "c"} {
		got, ok := f.next(context.Background())
		if !ok || got.url != want {
			t.Fatalf("next() = %+v, ok=%v; want %q", got, ok, want)
		}
		f.taskDone()
	}
}

// TestFrontierExhaustsWhenEmptyAndNothingPending guards the termination condition: once every
// pushed task has been popped and marked done, next() must report exhaustion rather than
// blocking forever.
func TestFrontierExhaustsWhenEmptyAndNothingPending(t *testing.T) {
	f := newFrontier()
	f.push(task{url: "a"})
	got, ok := f.next(context.Background())
	if !ok || got.url != "a" {
		t.Fatalf("next() = %+v, ok=%v", got, ok)
	}
	f.taskDone()

	done := make(chan bool, 1)
	go func() {
		_, ok := f.next(context.Background())
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Error("expected next() to report exhaustion, got a task")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("next() blocked instead of reporting exhaustion")
	}
}

// TestFrontierWaitsForPendingWork guards against premature exhaustion: a worker blocked in
// next() must wait for a task pushed by another still-processing task (pending > 0, queue
// empty), not report exhaustion just because the queue happened to be empty for a moment.
func TestFrontierWaitsForPendingWork(t *testing.T) {
	f := newFrontier()
	f.push(task{url: "a"})
	first, ok := f.next(context.Background())
	if !ok || first.url != "a" {
		t.Fatalf("next() = %+v, ok=%v", first, ok)
	}
	// Queue is now empty, but "a" is still pending (taskDone hasn't been called), so a second
	// next() must block rather than report exhaustion.
	second := make(chan task, 1)
	go func() {
		t, ok := f.next(context.Background())
		if ok {
			second <- t
		}
	}()

	select {
	case <-second:
		t.Fatal("next() returned before the pending task pushed a child")
	case <-time.After(50 * time.Millisecond):
	}

	f.push(task{url: "b"}) // simulates "a" discovering a child before finishing
	f.taskDone()           // "a" is now done

	select {
	case got := <-second:
		if got.url != "b" {
			t.Errorf("got task %q, want %q", got.url, "b")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked next() was never woken by the push")
	}
}

// TestFrontierNextRespectsCancellation guards against a canceled context leaving workers
// blocked forever: a worker waiting in next() must be woken and exit promptly once ctx is
// done, even though nothing pushed or finished a task.
func TestFrontierNextRespectsCancellation(t *testing.T) {
	f := newFrontier()
	f.push(task{url: "a"})
	if _, ok := f.next(context.Background()); !ok {
		t.Fatal("expected the first next() to return the pushed task")
	}
	// "a" is still pending, so without cancellation this would block indefinitely.
	ctx, cancel := context.WithCancel(context.Background())
	stop := f.watchCancellation(ctx)
	defer stop()

	result := make(chan bool, 1)
	go func() {
		_, ok := f.next(ctx)
		result <- ok
	}()

	time.Sleep(20 * time.Millisecond) // let the goroutine reach next() and block
	cancel()

	select {
	case ok := <-result:
		if ok {
			t.Error("expected next() to report exhaustion after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("next() did not wake up after ctx was canceled")
	}
}

func TestFrontierPausedBlocksNextUntilResumed(t *testing.T) {
	f := newFrontier()
	f.push(task{url: "a"})
	f.setPaused(true)
	got := make(chan task, 1)
	go func() {
		if tk, ok := f.next(context.Background()); ok {
			got <- tk
		}
	}()
	select {
	case tk := <-got:
		t.Fatalf("next returned %q while paused", tk.url)
	case <-time.After(100 * time.Millisecond):
	}
	f.setPaused(false)
	select {
	case tk := <-got:
		if tk.url != "a" {
			t.Fatalf("next = %q, want a", tk.url)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("next did not wake when the frontier resumed")
	}
}

func TestFrontierPausedStillWakesOnCancel(t *testing.T) {
	f := newFrontier()
	f.push(task{url: "a"})
	f.setPaused(true)
	ctx, cancel := context.WithCancel(context.Background())
	stop := f.watchCancellation(ctx)
	defer stop()
	done := make(chan bool, 1)
	go func() {
		_, ok := f.next(ctx)
		done <- ok
	}()
	cancel()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("next returned a task after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not wake a paused next()")
	}
}

func TestFrontierPausedWithNothingPendingIsExhausted(t *testing.T) {
	f := newFrontier()
	f.setPaused(true)
	done := make(chan bool, 1)
	go func() {
		_, ok := f.next(context.Background())
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("next returned a task from an empty frontier")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a paused, empty frontier must still report exhaustion")
	}
}

func TestFrontierPushFrontRequeuesAtHead(t *testing.T) {
	f := newFrontier()
	ctx := context.Background()
	f.push(task{url: "a"})
	f.push(task{url: "b"})
	a, _ := f.next(ctx)
	f.pushFront(a) // the worker hands the task back...
	f.taskDone()   // ...then ends its turn for it
	for _, want := range []string{"a", "b"} {
		got, ok := f.next(ctx)
		if !ok || got.url != want {
			t.Fatalf("next = %+v ok=%v, want %q", got, ok, want)
		}
		f.taskDone()
	}
	if _, ok := f.next(ctx); ok {
		t.Fatal("expected exhaustion after both tasks")
	}
}
