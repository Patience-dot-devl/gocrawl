package crawler

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Adaptive-delay tuning. When the server signals overload (HTTP 429/503) the limiter halves
// its requests-per-second down to backoffMinRate, ignoring repeat triggers that arrive within
// backoffDebounce of the last adjustment (a single burst of 429s should back off once, not
// once per concurrent worker).
const (
	backoffStartRate = 1.0             // req/s to drop to when the crawl was previously unrestricted
	backoffMinRate   = 0.1             // floor: one request every 10s
	backoffDebounce  = 2 * time.Second // ignore repeat triggers within this window
	// maxRetryAfter bounds how long a server-supplied Retry-After is honored for. A
	// misconfigured or hostile server can send an enormous value (e.g. Retry-After: 86400)
	// which would otherwise stall the entire crawl for as long as it asks, with no overall
	// deadline to recover; five minutes is generous headroom for a genuine rate-limit window
	// while still bounding the worst case.
	maxRetryAfter = 5 * time.Minute
)

// AdaptiveLimiter is a requests-per-second limiter that halves its rate (down to a floor)
// whenever a fetch reports HTTP 429 or 503, honoring any Retry-After header, and debounces
// bursts of concurrent triggers into a single adjustment. It is shared by anything that fetches
// a live site directly — the crawl Engine and tools like check-redirects — so a run that starts
// too aggressively for the target backs off instead of hammering it into a bot-mitigation block.
type AdaptiveLimiter struct {
	limiter *rate.Limiter
	enabled bool

	mu            sync.Mutex
	baseRate      float64 // configured req/s (0 = unlimited)
	curRate       float64 // current req/s after any backoff (0 = still unlimited)
	lastAdjust    time.Time
	throttleCount int
}

// NewAdaptiveLimiter creates a limiter starting at ratePerSecond (0 = unlimited). Wait always
// enforces the current rate; when enabled is false, OnResponse never backs it off.
func NewAdaptiveLimiter(ratePerSecond float64, enabled bool) *AdaptiveLimiter {
	limit := rate.Inf
	if ratePerSecond > 0 {
		limit = rate.Limit(ratePerSecond)
	}
	return &AdaptiveLimiter{
		limiter:  rate.NewLimiter(limit, 1),
		enabled:  enabled,
		baseRate: ratePerSecond,
	}
}

// Wait blocks until the current rate allows another request.
func (a *AdaptiveLimiter) Wait(ctx context.Context) error {
	return a.limiter.Wait(ctx)
}

// OnResponse inspects a fetch outcome and, if it signals overload (429/503), halves the current
// rate down to backoffMinRate, never faster than any Retry-After header demands. It reports
// whether this call triggered an adjustment (false if disabled, the status wasn't 429/503, or a
// trigger already landed within backoffDebounce) and the resulting rate.
func (a *AdaptiveLimiter) OnResponse(statusCode int, header http.Header) (adjusted bool, newRate float64) {
	if !a.enabled {
		return false, 0
	}
	if statusCode != http.StatusTooManyRequests && statusCode != http.StatusServiceUnavailable {
		return false, 0
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	if !a.lastAdjust.IsZero() && now.Sub(a.lastAdjust) < backoffDebounce {
		return false, 0
	}
	a.lastAdjust = now

	prev := a.curRate
	if prev <= 0 {
		prev = a.baseRate
	}
	var next float64
	if prev <= 0 {
		next = backoffStartRate // was previously unrestricted
	} else {
		next = prev / 2
	}
	// The floor only bounds our halving heuristic.
	if next < backoffMinRate {
		next = backoffMinRate
	}
	// An explicit Retry-After is a direct server instruction, so honor it even below the
	// heuristic floor: never go faster than it asks.
	if ra := retryAfterSeconds(header); ra > 0 {
		if byRetry := 1.0 / ra; byRetry < next {
			next = byRetry
		}
	}
	a.curRate = next
	a.throttleCount++
	a.limiter.SetLimit(rate.Limit(next))
	return true, next
}

// ThrottleCount reports how many times OnResponse has backed off the rate.
func (a *AdaptiveLimiter) ThrottleCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.throttleCount
}

// CurrentRate reports the rate in effect after any backoff (0 if still unrestricted).
func (a *AdaptiveLimiter) CurrentRate() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.curRate
}

// retryAfterSeconds parses a Retry-After header, supporting both the delay-seconds and
// HTTP-date forms. It returns 0 when the header is absent, unparseable, or in the past, and
// clamps the result to maxRetryAfter so an extreme server-supplied value can't stall the crawl
// indefinitely.
func retryAfterSeconds(h http.Header) float64 {
	if h == nil {
		return 0
	}
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return math.Min(float64(secs), maxRetryAfter.Seconds())
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t).Seconds(); d > 0 {
			return math.Min(d, maxRetryAfter.Seconds())
		}
	}
	return 0
}

// AdaptiveFetcher wraps a Fetcher so every call waits on Limiter beforehand and lets it observe
// the response afterward. This is how tools that fetch a bounded list of URLs directly (rather
// than crawling through Engine) still back off politely on 429/503.
type AdaptiveFetcher struct {
	Fetcher Fetcher
	Limiter *AdaptiveLimiter
}

// Fetch implements Fetcher.
func (f *AdaptiveFetcher) Fetch(ctx context.Context, rawURL string) (*Page, error) {
	if err := f.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	page, err := f.Fetcher.Fetch(ctx, rawURL)
	if page != nil {
		f.Limiter.OnResponse(page.StatusCode, page.Header)
	}
	return page, err
}
