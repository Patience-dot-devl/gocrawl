package webserver

import (
	"net/http"
	"testing"
	"time"
)

func TestCloseCancelsRunningAndPausedCrawls(t *testing.T) {
	srv := newTestServer(t)
	site := slowSite(t, 200, 30*time.Millisecond)
	running := startSlowCrawl(t, srv, site)
	paused := startSlowCrawl(t, srv, site)
	pauseAndWait(t, srv, paused)

	start := time.Now()
	srv.Close()
	if took := time.Since(start); took >= closeGrace {
		t.Fatalf("Close took %v, want it to return once the jobs ended (grace %v)", took, closeGrace)
	}

	for name, id := range map[string]string{"running": running, "paused": paused} {
		_, job := doJSON(t, srv, http.MethodGet, "/api/crawls/"+id, nil)
		if job["status"] != "canceled" {
			t.Errorf("%s crawl: status = %v right after Close, want canceled", name, job["status"])
		}
	}
}

func TestCloseCancelsRedirectChecks(t *testing.T) {
	srv := newTestServer(t)
	blocked := make(chan struct{})
	job, ok := srv.redirects.start(redirectJob{Domain: "example.com"}, func() { close(blocked) }, 1)
	if !ok {
		t.Fatal("start refused")
	}
	go func() {
		<-blocked
		srv.redirects.finish(job.ID, nil, nil)
	}()
	srv.Close()
	got, _ := srv.redirects.get(job.ID)
	if got.Status != StatusCanceled {
		t.Errorf("redirect check status = %q after Close, want canceled", got.Status)
	}
}

func TestCloseDoesNotWaitForeverOnAStuckJob(t *testing.T) {
	old := closeGrace
	closeGrace = 100 * time.Millisecond
	t.Cleanup(func() { closeGrace = old })

	srv := newTestServer(t)
	if _, ok := srv.crawls.start(crawlJob{}, func() {}, 1); !ok {
		t.Fatal("start refused")
	}
	start := time.Now()
	srv.Close()
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("Close took %v with a stuck job, want it bounded by the grace period", took)
	}
}
