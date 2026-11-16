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
// slot for its host under the lock and then sleeps outside it, so concurrent callers that hit
// the same host queue up in order instead of all waking at once. Different hosts never wait
// on each other, and busy lets the crawl's frontier keep other workers off a host that a
// worker is already waiting on. hostPacer.mu is a leaf lock.
type hostPacer struct {
	mu      sync.Mutex
	next    map[string]time.Time // host -> earliest time the next request may start
	waiting map[string]int       // host -> callers sleeping in wait right now
	now     func() time.Time
}

func newHostPacer() *hostPacer {
	return &hostPacer{next: make(map[string]time.Time), waiting: make(map[string]int), now: time.Now}
}

// reserve books the next slot for host and returns how long the caller must wait for it.
// delay <= 0 books nothing and returns 0. delay is capped at MaxCrawlDelay.
func (p *hostPacer) reserve(host string, delay time.Duration) time.Duration {
	return p.book(host, delay, false)
}

// book is reserve; with hold set, a caller that must wait is also counted in waiting, in the
// same critical section, so busy reports the host from the moment its slot is booked.
func (p *hostPacer) book(host string, delay time.Duration, hold bool) time.Duration {
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
	d := slot.Sub(now)
	if hold && d > 0 {
		p.waiting[host]++
	}
	return d
}

// busy reports whether a caller is sleeping in wait for host's next slot.
func (p *hostPacer) busy(host string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waiting[host] > 0
}

// wait blocks until host's next slot (see reserve), or until ctx is done, which it returns.
// slept reports whether it had to wait; while it sleeps, busy(host) is true.
func (p *hostPacer) wait(ctx context.Context, host string, delay time.Duration) (slept bool, err error) {
	d := p.book(host, delay, true)
	if d <= 0 {
		return false, ctx.Err()
	}
	defer func() {
		p.mu.Lock()
		if p.waiting[host]--; p.waiting[host] <= 0 {
			delete(p.waiting, host)
		}
		p.mu.Unlock()
	}()
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true, nil
	case <-ctx.Done():
		return true, ctx.Err()
	}
}
