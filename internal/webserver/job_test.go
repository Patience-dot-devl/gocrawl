package webserver

import (
	"errors"
	"testing"
)

type testJobData struct {
	N int
}

func noopCancel() {}

func TestJobManagerCapCountsRunningAndPausedJobs(t *testing.T) {
	m := newJobManager[testJobData]()
	first, ok := m.start(testJobData{N: 1}, noopCancel, 2)
	if !ok {
		t.Fatal("first start refused")
	}
	if _, ok := m.start(testJobData{N: 2}, noopCancel, 2); !ok {
		t.Fatal("second start refused")
	}
	m.with(first.ID, func(j *Job[testJobData]) { j.Status = StatusPaused })
	if _, ok := m.start(testJobData{N: 3}, noopCancel, 2); ok {
		t.Fatal("start accepted with one running and one paused job at cap 2; paused jobs must count")
	}
	m.finish(first.ID, nil, nil)
	if _, ok := m.start(testJobData{N: 3}, noopCancel, 2); !ok {
		t.Fatal("start refused after the paused job finished")
	}
}

func TestJobManagerStartReturnsRunningSnapshot(t *testing.T) {
	m := newJobManager[testJobData]()
	j, ok := m.start(testJobData{N: 7}, noopCancel, 1)
	if !ok {
		t.Fatal("start refused")
	}
	if j.ID == "" || j.Status != StatusRunning || j.StartedAt.IsZero() || j.Data.N != 7 {
		t.Fatalf("start returned %+v, want an id, status running, a start time and the data", j)
	}
}

func TestJobManagerWithMutatesLiveJob(t *testing.T) {
	m := newJobManager[testJobData]()
	j, _ := m.start(testJobData{N: 1}, noopCancel, 1)
	before, _ := m.get(j.ID)

	if !m.with(j.ID, func(live *Job[testJobData]) { live.Data.N = 42 }) {
		t.Fatal("with returned false for a known job")
	}
	after, _ := m.get(j.ID)
	if after.Data.N != 42 {
		t.Fatalf("Data.N = %d after with, want 42", after.Data.N)
	}
	if before.Data.N != 1 {
		t.Fatalf("earlier snapshot changed to %d; get must return a copy", before.Data.N)
	}
	if m.with("missing", func(*Job[testJobData]) { t.Error("fn called for an unknown id") }) {
		t.Fatal("with returned true for an unknown id")
	}
}

func TestJobManagerFinishAppliesData(t *testing.T) {
	m := newJobManager[testJobData]()
	ok1, _ := m.start(testJobData{}, noopCancel, 2)
	bad, _ := m.start(testJobData{}, noopCancel, 2)

	m.finish(ok1.ID, nil, func(d *testJobData) { d.N = 5 })
	got, _ := m.get(ok1.ID)
	if got.Status != StatusDone || got.Data.N != 5 || got.FinishedAt.IsZero() || got.Err != "" {
		t.Fatalf("after a clean finish: %+v, want done, N=5, a finish time and no error", got)
	}

	m.finish(bad.ID, errors.New("boom"), func(d *testJobData) { d.N = 9 })
	got, _ = m.get(bad.ID)
	if got.Status != StatusError || got.Err != "boom" || got.Data.N != 9 {
		t.Fatalf("after a failed finish: %+v, want error \"boom\" and apply still run (N=9)", got)
	}

	m.finish("missing", nil, func(*testJobData) { t.Error("apply called for an unknown id") })
}

func TestJobManagerCancel(t *testing.T) {
	m := newJobManager[testJobData]()
	cancelled := 0
	cancel := func() { cancelled++ }

	running, _ := m.start(testJobData{}, cancel, 3)
	paused, _ := m.start(testJobData{}, cancel, 3)
	m.with(paused.ID, func(j *Job[testJobData]) { j.Status = StatusPaused })
	done, _ := m.start(testJobData{}, cancel, 3)
	m.finish(done.ID, nil, nil)

	if !m.cancel(running.ID) {
		t.Error("cancel(running) = false, want true")
	}
	if !m.cancel(paused.ID) {
		t.Error("cancel(paused) = false, want true")
	}
	if m.cancel(done.ID) {
		t.Error("cancel(done) = true, want false")
	}
	if m.cancel("missing") {
		t.Error("cancel(unknown) = true, want false")
	}
	if cancelled != 2 {
		t.Errorf("cancel func called %d times, want 2", cancelled)
	}

	m.finish(running.ID, nil, nil)
	m.finish(paused.ID, nil, nil)
	for _, id := range []string{running.ID, paused.ID} {
		if j, _ := m.get(id); j.Status != StatusCanceled {
			t.Errorf("job %s status = %s after cancel + finish, want canceled", id, j.Status)
		}
	}
}

func TestJobManagerList(t *testing.T) {
	m := newJobManager[testJobData]()
	m.start(testJobData{N: 1}, noopCancel, 5)
	m.start(testJobData{N: 2}, noopCancel, 5)
	if got := len(m.list()); got != 2 {
		t.Fatalf("list returned %d jobs, want 2", got)
	}
	if _, ok := m.get("missing"); ok {
		t.Fatal("get(unknown) ok = true")
	}
}
