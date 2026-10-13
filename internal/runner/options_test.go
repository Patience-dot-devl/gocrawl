package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

func TestNewRunSettingsAppliesEveryOption(t *testing.T) {
	calls := 0
	count := RunOption(func(*runSettings) { calls++ })
	newRunSettings([]RunOption{count, nil, count})
	if calls != 2 {
		t.Fatalf("options applied %d times, want 2 (nil options are skipped)", calls)
	}
}

// TestRunAcceptsOptions: Run takes RunOptions as a variadic tail, so the CLI and MCP callers
// compile unchanged, and it applies each one exactly once per call.
func TestRunAcceptsOptions(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<html><head><title>Home</title></head><body>hello</body></html>")
	}))
	defer site.Close()

	cfg := config.Default()
	cfg.Crawl.MaxDepth = 0
	cfg.Crawl.MaxPages = 1

	calls := 0
	opt := RunOption(func(*runSettings) { calls++ })
	rep, err := Run(context.Background(), cfg, site.URL, opt)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.PagesCrawled != 1 {
		t.Errorf("PagesCrawled = %d, want 1", rep.PagesCrawled)
	}
	if calls != 1 {
		t.Errorf("option applied %d times, want 1", calls)
	}
}
