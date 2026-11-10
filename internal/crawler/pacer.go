package crawler

import (
	"context"
	"sync"
	"time"
)

// MaxCrawlDelay caps the robots.txt Crawl-delay the crawler obeys. Some sites set values of
// minutes or hours; obeying those literally makes a crawl look hung. A crawl that must go
// slower can still set a lower RatePerSecond.
const MaxCrawlDelay = 60 * time.Second

// hostPacer spaces requests to one host at least delay apart: the per-host politeness that
// robots.txt Crawl-delay asks for. It works by reservation: each caller takes the next free
// slot for its host under the lock and then sleeps outside it, so concurrent workers that hit
// the same host queue up in order instead of all waking at once. Different hosts never wait
// on each other. hostPacer.mu is a leaf lock.
type hostPacer struct {
	mu   sync.Mutex
	next map[string]time.Time // host -> earliest time the next request may start
	now  func() time.Time
}

func newHostPacer() *hostPacer {
	return &hostPacer{next: make(map[string]time.Time), now: time.Now}
}

// reserve books the next slot for host and returns how long the caller must wait for it.
// delay <= 0 books nothing and returns 0. delay is capped at MaxCrawlDelay.
func (p *hostPacer) reserve(host string, delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	if delay > MaxCrawlDelay {
		delay = MaxCrawlDelay
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	slot := p.next[host]
	if slot.Before(now) {
		slot = now
	}
	p.next[host] = slot.Add(delay)
	return slot.Sub(now)
}

// wait blocks until host's next slot (see reserve), or until ctx is done, which it returns.
func (p *hostPacer) wait(ctx context.Context, host string, delay time.Duration) error {
	d := p.reserve(host, delay)
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
