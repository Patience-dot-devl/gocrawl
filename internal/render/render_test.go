package render

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// hasBrowser reports whether a Chromium-class binary is available on PATH so headless
// tests can run.
func hasBrowser() bool {
	for _, name := range []string{
		"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
		"Google Chrome", "msedge",
	} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	// macOS bundle locations not on PATH.
	for _, p := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	} {
		if _, err := exec.LookPath(p); err == nil {
			return true
		}
	}
	return false
}

func TestHeadlessFetchCapturesCWV(t *testing.T) {
	if !hasBrowser() {
		t.Skip("no Chromium-class browser available on PATH")
	}

	const html = `<!doctype html>
<html><head><title>t</title></head>
<body>
<h1>Hello</h1>
<img src="data:image/svg+xml;utf8,%3Csvg xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22 width%3D%22400%22 height%3D%22200%22%3E%3Crect width%3D%22400%22 height%3D%22200%22 fill%3D%22%2399ccff%22%2F%3E%3C%2Fsvg%3E" alt="x">
<script>
  // Simulate a small main-thread block so longtask/TBT can surface (best-effort).
  const t = Date.now();
  while (Date.now() - t < 80) { /* spin */ }
</script>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	opts := crawler.DefaultOptions()
	opts.Timeout = 30 * time.Second
	f, err := NewHeadlessFetcher(opts)
	if err != nil {
		t.Skipf("could not launch headless browser: %v", err)
	}
	defer f.Close()

	page, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if page == nil {
		t.Fatal("nil page")
	}
	if page.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", page.StatusCode)
	}
	if page.Render == nil || !page.Render.Implemented {
		t.Fatalf("expected Implemented=true Render, got %+v", page.Render)
	}
	if page.Render.TTFB <= 0 {
		t.Errorf("expected TTFB > 0, got %v", page.Render.TTFB)
	}
	if len(page.Body) == 0 {
		t.Error("expected non-empty body")
	}
	if !page.IsHTML() {
		t.Error("expected parsed Doc")
	}
}

func TestUnderRendered(t *testing.T) {
	cases := []struct {
		name          string
		rendered, raw int
		want          bool
	}{
		{"render lost most of the page", 1_000, 100_000, true}, // the staging-site bug
		{"render matches raw (SSR)", 100_000, 100_000, false},
		{"render richer than raw (SPA hydration)", 200_000, 10_000, false},
		{"tiny raw is ignored", 10, 1_500, false},          // below the minimum-raw guard
		{"just over half is fine", 60_000, 100_000, false}, // 60% of a big page
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := underRendered(c.rendered, c.raw); got != c.want {
				t.Errorf("underRendered(%d, %d) = %v, want %v", c.rendered, c.raw, got, c.want)
			}
		})
	}
}
