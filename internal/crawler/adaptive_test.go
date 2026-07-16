package crawler

import (
	"context"
	"net/http"
	"testing"
)

func TestAdaptiveLimiterOnResponse(t *testing.T) {
	limiter := NewAdaptiveLimiter(0, true)

	// First 429 on an unrestricted limiter drops to the start rate.
	adjusted, next := limiter.OnResponse(http.StatusTooManyRequests, nil)
	if !adjusted || next != backoffStartRate {
		t.Fatalf("first 429: adjusted=%v next=%v, want true/%v", adjusted, next, backoffStartRate)
	}
	if got := float64(limiter.limiter.Limit()); got != backoffStartRate {
		t.Fatalf("after first 429: limit=%v, want %v", got, backoffStartRate)
	}

	// A repeat within the debounce window is ignored.
	if adjusted, _ := limiter.OnResponse(http.StatusTooManyRequests, nil); adjusted {
		t.Fatal("repeat within debounce window should be ignored")
	}
	if got := float64(limiter.limiter.Limit()); got != backoffStartRate {
		t.Fatalf("repeat within debounce changed limit to %v", got)
	}

	// Force the next adjustment past the debounce window; rate halves.
	limiter.lastAdjust = limiter.lastAdjust.Add(-2 * backoffDebounce)
	adjusted, next = limiter.OnResponse(http.StatusTooManyRequests, nil)
	if !adjusted || next != backoffStartRate/2 {
		t.Fatalf("after second 429: adjusted=%v next=%v, want true/%v", adjusted, next, backoffStartRate/2)
	}
	if got := limiter.ThrottleCount(); got != 2 {
		t.Fatalf("ThrottleCount=%v, want 2", got)
	}
}

func TestAdaptiveLimiterDisabled(t *testing.T) {
	limiter := NewAdaptiveLimiter(5, false)
	if adjusted, _ := limiter.OnResponse(http.StatusTooManyRequests, nil); adjusted {
		t.Fatal("disabled adaptive delay should never adjust")
	}
	if got := float64(limiter.limiter.Limit()); got != 5 {
		t.Fatalf("disabled adaptive delay still changed limit to %v", got)
	}
}

func TestAdaptiveLimiterIgnoresNon429503(t *testing.T) {
	limiter := NewAdaptiveLimiter(5, true)
	if adjusted, _ := limiter.OnResponse(http.StatusOK, nil); adjusted {
		t.Fatal("a 200 response should never trigger a backoff")
	}
}

func TestAdaptiveLimiterHonorsRetryAfter(t *testing.T) {
	limiter := NewAdaptiveLimiter(4, true) // would otherwise halve to 2 req/s
	header := http.Header{}
	header.Set("Retry-After", "20") // asks for 0.05 req/s

	_, next := limiter.OnResponse(http.StatusServiceUnavailable, header)
	if next != 0.05 {
		t.Fatalf("Retry-After not honored: next=%v, want 0.05", next)
	}
	if got := float64(limiter.limiter.Limit()); got != 0.05 {
		t.Fatalf("Retry-After not honored: limit=%v, want 0.05", got)
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	cases := []struct {
		val  string
		want float64
	}{
		{"", 0},
		{"30", 30},
		{"-5", 0},
		{"garbage", 0},
	}
	for _, c := range cases {
		h := http.Header{}
		if c.val != "" {
			h.Set("Retry-After", c.val)
		}
		if got := retryAfterSeconds(h); got != c.want {
			t.Errorf("retryAfterSeconds(%q)=%v, want %v", c.val, got, c.want)
		}
	}
	if got := retryAfterSeconds(nil); got != 0 {
		t.Errorf("retryAfterSeconds(nil)=%v, want 0", got)
	}
}

func TestAdaptiveFetcherThrottlesOnResponse(t *testing.T) {
	limiter := NewAdaptiveLimiter(0, true)
	fetcher := &AdaptiveFetcher{
		Fetcher: fakeFetcherFunc(func(context.Context, string) (*Page, error) {
			return &Page{StatusCode: http.StatusTooManyRequests}, nil
		}),
		Limiter: limiter,
	}
	if _, err := fetcher.Fetch(context.Background(), "https://example.test/"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := limiter.ThrottleCount(); got != 1 {
		t.Fatalf("ThrottleCount after one 429 fetch = %v, want 1", got)
	}
}

type fakeFetcherFunc func(ctx context.Context, rawURL string) (*Page, error)

func (f fakeFetcherFunc) Fetch(ctx context.Context, rawURL string) (*Page, error) {
	return f(ctx, rawURL)
}
