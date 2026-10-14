package crawlrequest

import (
	"strings"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

func TestToConfig_RequiresURL(t *testing.T) {
	_, _, err := Params{}.ToConfig()
	if err == nil {
		t.Fatal("expected an error for an empty URL")
	}
}

func TestToConfig_DefaultsSchemeToHTTPS(t *testing.T) {
	_, seed, err := Params{URL: "example.com"}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if seed != "https://example.com" {
		t.Errorf("seed = %q, want https://example.com", seed)
	}
}

func TestToConfig_OverridesApply(t *testing.T) {
	depth := 3
	maxPages := 42
	concurrency := 8
	rate := 2.5
	specialized := true
	respectRobots := false
	subdomains := true
	followExternal := true

	cfg, seed, err := Params{
		URL:            "https://example.com",
		Depth:          &depth,
		MaxPages:       &maxPages,
		Concurrency:    &concurrency,
		RatePerSecond:  &rate,
		Render:         "headless",
		Analyzers:      []string{"seo", "links"},
		Specialized:    &specialized,
		RespectRobots:  &respectRobots,
		Subdomains:     &subdomains,
		FollowExternal: &followExternal,
		Include:        []string{"^/blog"},
		Exclude:        []string{"^/admin"},
		UserAgent:      "test-agent",
	}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if seed != "https://example.com" {
		t.Errorf("seed = %q, want https://example.com", seed)
	}
	if cfg.Crawl.MaxDepth != depth {
		t.Errorf("MaxDepth = %d, want %d", cfg.Crawl.MaxDepth, depth)
	}
	if cfg.Crawl.MaxPages != maxPages {
		t.Errorf("MaxPages = %d, want %d", cfg.Crawl.MaxPages, maxPages)
	}
	if cfg.Crawl.Concurrency != concurrency {
		t.Errorf("Concurrency = %d, want %d", cfg.Crawl.Concurrency, concurrency)
	}
	if cfg.Crawl.RatePerSecond != rate {
		t.Errorf("RatePerSecond = %v, want %v", cfg.Crawl.RatePerSecond, rate)
	}
	if cfg.Render != "headless" {
		t.Errorf("Render = %q, want headless", cfg.Render)
	}
	if !cfg.Analyzers.Specialized {
		t.Error("Specialized = false, want true")
	}
	if cfg.Crawl.RespectRobots {
		t.Error("RespectRobots = true, want false")
	}
	if !cfg.Crawl.AllowSubdomains {
		t.Error("AllowSubdomains = false, want true")
	}
	if !cfg.Crawl.FollowExternal {
		t.Error("FollowExternal = false, want true")
	}
	if len(cfg.Crawl.Include) != 1 || cfg.Crawl.Include[0] != "^/blog" {
		t.Errorf("Include = %v", cfg.Crawl.Include)
	}
	if cfg.Crawl.UserAgent != "test-agent" {
		t.Errorf("UserAgent = %q, want test-agent", cfg.Crawl.UserAgent)
	}
}

func TestToConfig_InvalidMaxDuration(t *testing.T) {
	_, _, err := Params{URL: "https://example.com", MaxDuration: "not-a-duration"}.ToConfig()
	if err == nil {
		t.Fatal("expected an error for an invalid max_duration")
	}
}

func TestToConfig_SeedUserinfoBecomesBasicAuth(t *testing.T) {
	cfg, seed, err := Params{URL: "https://user:pass@example.com"}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if seed != "https://example.com" {
		t.Errorf("seed = %q, want userinfo stripped", seed)
	}
	if cfg.Crawl.BasicAuth != "user:pass" {
		t.Errorf("BasicAuth = %q, want user:pass", cfg.Crawl.BasicAuth)
	}
}

func TestToConfig_MapsCookie(t *testing.T) {
	cfg, _, err := Params{URL: "https://example.com", Cookie: "storefront_digest=abc123"}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if cfg.Crawl.Cookie != "storefront_digest=abc123" {
		t.Errorf("Cookie = %q, want storefront_digest=abc123", cfg.Crawl.Cookie)
	}
}

func TestToConfig_MapsHeadersAndProfile(t *testing.T) {
	cfg, _, err := Params{
		URL:           "https://example.com",
		Headers:       []string{"X-Token: abc"},
		HeaderProfile: "uk",
	}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if len(cfg.Crawl.Headers) != 1 || cfg.Crawl.Headers[0] != "X-Token: abc" {
		t.Errorf("Headers = %q, want [X-Token: abc]", cfg.Crawl.Headers)
	}
	if cfg.Crawl.HeaderProfile != "uk" {
		t.Errorf("HeaderProfile = %q, want uk", cfg.Crawl.HeaderProfile)
	}
	if cfg.Crawl.HeaderFile != "" {
		t.Errorf("HeaderFile = %q, want empty (not settable from a request)", cfg.Crawl.HeaderFile)
	}
}

func TestToConfig_ExplicitBasicAuthWinsOverSeedUserinfo(t *testing.T) {
	cfg, _, err := Params{URL: "https://user:pass@example.com", BasicAuth: "explicit:auth"}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if cfg.Crawl.BasicAuth != "explicit:auth" {
		t.Errorf("BasicAuth = %q, want explicit:auth", cfg.Crawl.BasicAuth)
	}
}

func TestToConfig_IgnoreExternalTaggingDefaultsTrue(t *testing.T) {
	cfg, _, err := Params{URL: "https://example.com"}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if !cfg.Analyzers.IgnoreExternalTagging {
		t.Error("IgnoreExternalTagging = false, want true (default)")
	}
}

func TestToConfig_IgnoreExternalTaggingOverride(t *testing.T) {
	ignore := false
	cfg, _, err := Params{URL: "https://example.com", IgnoreExternalTagging: &ignore}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if cfg.Analyzers.IgnoreExternalTagging {
		t.Error("IgnoreExternalTagging = true, want false (explicit override)")
	}
}

func TestToConfig_MapsCrawlBehaviorParams(t *testing.T) {
	adaptive := false
	cfg, _, err := Params{
		URL:            "https://example.com",
		StripQuery:     true,
		AdaptiveDelay:  &adaptive,
		FollowNofollow: true,
		Timeout:        " 7s ",
		BearerToken:    " tok-123 ",
	}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if !cfg.Crawl.StripQuery {
		t.Error("StripQuery = false, want true")
	}
	if cfg.Crawl.AdaptiveDelay {
		t.Error("AdaptiveDelay = true, want false (explicit override)")
	}
	if !cfg.Crawl.FollowNofollow {
		t.Error("FollowNofollow = false, want true")
	}
	if cfg.Crawl.Timeout != 7*time.Second {
		t.Errorf("Timeout = %v, want 7s", cfg.Crawl.Timeout)
	}
	if cfg.Crawl.BearerToken != "tok-123" {
		t.Errorf("BearerToken = %q, want tok-123 (trimmed)", cfg.Crawl.BearerToken)
	}
}

func TestToConfig_CrawlBehaviorDefaults(t *testing.T) {
	cfg, _, err := Params{URL: "https://example.com"}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	d := config.Default()
	if !cfg.Crawl.AdaptiveDelay || cfg.Crawl.AdaptiveDelay != d.Crawl.AdaptiveDelay {
		t.Error("AdaptiveDelay should default to the config default (true)")
	}
	if cfg.Crawl.Timeout != d.Crawl.Timeout {
		t.Errorf("Timeout = %v, want the default %v", cfg.Crawl.Timeout, d.Crawl.Timeout)
	}
	if cfg.Crawl.StripQuery || cfg.Crawl.FollowNofollow || cfg.Crawl.BearerToken != "" {
		t.Errorf("unexpected non-default: strip_query=%v follow_nofollow=%v bearer set=%v",
			cfg.Crawl.StripQuery, cfg.Crawl.FollowNofollow, cfg.Crawl.BearerToken != "")
	}
}

func TestToConfig_InvalidTimeout(t *testing.T) {
	for _, v := range []string{"soon", "-1s", "0s"} {
		if _, _, err := (Params{URL: "https://example.com", Timeout: v}).ToConfig(); err == nil {
			t.Errorf("timeout %q: expected an error", v)
		}
	}
}

func TestToConfig_RejectsUnparsableSeedWithoutEchoingIt(t *testing.T) {
	for _, raw := range []string{
		"https://alice:s3cret@example.com/%zz",
		"alice:s3cret@example.com/%zz",
	} {
		_, _, err := Params{URL: raw}.ToConfig()
		if err == nil {
			t.Fatalf("ToConfig(%q): expected an error for an unparsable URL", raw)
		}
		if strings.Contains(err.Error(), "s3cret") || strings.Contains(err.Error(), "alice") {
			t.Errorf("error %q echoes the seed's userinfo", err)
		}
	}
}
