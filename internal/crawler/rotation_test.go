package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

func TestParseRotation(t *testing.T) {
	cases := map[string]RotationStrategy{
		"":            RotateRoundRobin,
		"round-robin": RotateRoundRobin,
		"rr":          RotateRoundRobin,
		"random":      RotateRandom,
		"sticky-host": RotateStickyHost,
		"sticky":      RotateStickyHost,
		"off":         RotateOff,
		"OFF":         RotateOff,
	}
	for in, want := range cases {
		got, err := ParseRotation(in)
		if err != nil {
			t.Errorf("ParseRotation(%q) unexpected error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseRotation(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseRotation("teleport"); err == nil {
		t.Error("ParseRotation(\"teleport\") expected an error, got nil")
	}
}

func TestUAPoolRoundRobin(t *testing.T) {
	p := NewUAPool(Options{UserAgents: []string{"a", "b", "c"}, UserAgentRotation: RotateRoundRobin})
	var got []string
	for i := 0; i < 6; i++ {
		got = append(got, p.Next("example.com"))
	}
	want := []string{"a", "b", "c", "a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("round-robin sequence = %v, want %v", got, want)
		}
	}
}

func TestUAPoolOffUsesFirst(t *testing.T) {
	p := NewUAPool(Options{UserAgents: []string{"a", "b", "c"}, UserAgentRotation: RotateOff})
	for i := 0; i < 4; i++ {
		if ua := p.Next("example.com"); ua != "a" {
			t.Fatalf("RotateOff returned %q, want \"a\"", ua)
		}
	}
}

func TestUAPoolStickyHost(t *testing.T) {
	p := NewUAPool(Options{UserAgents: []string{"a", "b", "c"}, UserAgentRotation: RotateStickyHost})
	first := p.Next("example.com")
	for i := 0; i < 5; i++ {
		if ua := p.Next("example.com"); ua != first {
			t.Fatalf("sticky-host returned %q then %q for the same host", first, ua)
		}
	}
	// A different host may map elsewhere; just assert determinism for it too.
	other := p.Next("other.test")
	if ua := p.Next("other.test"); ua != other {
		t.Fatalf("sticky-host not deterministic for other.test: %q then %q", other, ua)
	}
}

func TestUAPoolFallsBackToSingleUserAgent(t *testing.T) {
	p := NewUAPool(Options{UserAgent: "solo"})
	if ua := p.Next("x"); ua != "solo" {
		t.Fatalf("single UserAgent = %q, want \"solo\"", ua)
	}
	if p.Rotates() {
		t.Error("single-agent pool should report Rotates() == false")
	}
}

func TestProxyPoolRoundRobinSelection(t *testing.T) {
	mustURL := func(s string) *url.URL {
		u, err := url.Parse(s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return u
	}
	pool := newProxyPool(Options{
		Proxies:       []*url.URL{mustURL("http://p1:8080"), mustURL("http://p2:8080")},
		ProxyRotation: RotateRoundRobin,
	})
	if pool == nil {
		t.Fatal("expected a non-nil proxy pool")
	}
	pick := pool.proxyFunc()
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/page", nil)
	var hosts []string
	for i := 0; i < 4; i++ {
		u, err := pick(req)
		if err != nil {
			t.Fatalf("pick: %v", err)
		}
		hosts = append(hosts, u.Host)
	}
	want := []string{"p1:8080", "p2:8080", "p1:8080", "p2:8080"}
	for i := range want {
		if hosts[i] != want[i] {
			t.Fatalf("proxy round-robin = %v, want %v", hosts, want)
		}
	}
}

func TestNewProxyPoolNilWhenEmpty(t *testing.T) {
	if p := newProxyPool(Options{}); p != nil {
		t.Error("expected nil proxy pool when no proxies are configured")
	}
}

// TestFetchRotatesUserAgent drives the real Fetch path against a test server and asserts the
// User-Agent header rotates round-robin across requests.
func TestFetchRotatesUserAgent(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("User-Agent"))
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer srv.Close()

	f := NewHTTPFetcher(Options{
		UserAgents:        []string{"ua-one", "ua-two"},
		UserAgentRotation: RotateRoundRobin,
	})
	for i := 0; i < 4; i++ {
		if _, err := f.Fetch(context.Background(), srv.URL); err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"ua-one", "ua-two", "ua-one", "ua-two"}
	for i := range want {
		if i >= len(seen) || seen[i] != want[i] {
			t.Fatalf("server saw User-Agents %v, want %v", seen, want)
		}
	}
}

// TestFetchRoutesThroughProxy points the fetcher at a stub proxy and asserts the request is
// routed through it (the proxy sees an absolute-URI request for the target host).
func TestFetchRoutesThroughProxy(t *testing.T) {
	var mu sync.Mutex
	var proxied []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		proxied = append(proxied, r.Host)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>via proxy</body></html>"))
	}))
	defer proxy.Close()

	proxyURL, _ := url.Parse(proxy.URL)
	f := NewHTTPFetcher(Options{
		Proxies:       []*url.URL{proxyURL},
		ProxyRotation: RotateRoundRobin,
		UserAgent:     "test",
	})
	page, err := f.Fetch(context.Background(), "http://target.example/page")
	if err != nil {
		t.Fatalf("fetch via proxy: %v", err)
	}
	if page.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", page.StatusCode)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(proxied) == 0 {
		t.Fatal("proxy received no requests")
	}
	if proxied[0] != "target.example" {
		t.Fatalf("proxy saw host %q, want \"target.example\"", proxied[0])
	}
}
