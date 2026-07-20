package crawlrequest

import "testing"

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
	specialized := true
	respectRobots := false
	subdomains := true

	cfg, seed, err := Params{
		URL:           "https://example.com",
		Depth:         &depth,
		MaxPages:      &maxPages,
		Concurrency:   &concurrency,
		Render:        "headless",
		Analyzers:     []string{"seo", "links"},
		Specialized:   &specialized,
		RespectRobots: &respectRobots,
		Subdomains:    &subdomains,
		Include:       []string{"^/blog"},
		Exclude:       []string{"^/admin"},
		UserAgent:     "test-agent",
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

func TestToConfig_ExplicitBasicAuthWinsOverSeedUserinfo(t *testing.T) {
	cfg, _, err := Params{URL: "https://user:pass@example.com", BasicAuth: "explicit:auth"}.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if cfg.Crawl.BasicAuth != "explicit:auth" {
		t.Errorf("BasicAuth = %q, want explicit:auth", cfg.Crawl.BasicAuth)
	}
}
