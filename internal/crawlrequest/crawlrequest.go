// Package crawlrequest maps a user-facing crawl request onto a config.Config. It is the one
// seam shared by every entry point that lets a caller parameterize a crawl — today the MCP
// "crawl" tool and the web API — so the mapping (seed normalization, option overrides,
// basic-auth extraction) lives in exactly one place.
package crawlrequest

import (
	"fmt"
	"strings"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Params is a user-facing crawl request. Optional fields override config.Default().
type Params struct {
	URL           string   `json:"url" jsonschema:"Seed URL to crawl (e.g. https://example.com)"`
	Depth         *int     `json:"depth,omitempty" jsonschema:"Maximum link hops from the seed (default 0 = unlimited; the crawl is bounded by max_pages)"`
	MaxPages      *int     `json:"max_pages,omitempty" jsonschema:"Hard cap on the number of pages crawled (default 500)"`
	Concurrency   *int     `json:"concurrency,omitempty" jsonschema:"Number of parallel fetch workers (default 4)"`
	MaxDuration   string   `json:"max_duration,omitempty" jsonschema:"Wall-clock budget for the whole crawl as a Go duration string, e.g. '90m' (default unlimited); on expiry the crawl stops early and still returns a partial report"`
	Render        string   `json:"render,omitempty" jsonschema:"Rendering mode: 'raw' (default) or 'headless'"`
	Analyzers     []string `json:"analyzers,omitempty" jsonschema:"Subset of analyzer names to run; empty runs all"`
	Specialized   *bool    `json:"specialized,omitempty" jsonschema:"Enable opt-in specialized AI-search checks (AEO answer-lead, GEO quotable-density); off by default"`
	RespectRobots *bool    `json:"respect_robots,omitempty" jsonschema:"Obey robots.txt while crawling (default true)"`
	Subdomains    *bool    `json:"subdomains,omitempty" jsonschema:"Follow links to subdomains of the seed host"`
	Include       []string `json:"include,omitempty" jsonschema:"Only crawl URLs matching at least one of these regexes"`
	Exclude       []string `json:"exclude,omitempty" jsonschema:"Skip URLs matching any of these regexes"`

	UserAgent         string   `json:"user_agent,omitempty" jsonschema:"User-Agent header sent on every request"`
	UserAgents        []string `json:"user_agents,omitempty" jsonschema:"Pool of User-Agent strings to rotate across (supersedes user_agent)"`
	UserAgentRotation string   `json:"user_agent_rotation,omitempty" jsonschema:"Rotation across user_agents: off, round-robin, or random"`
	Proxy             string   `json:"proxy,omitempty" jsonschema:"Route requests through this proxy URL (http(s):// or socks5://; supports user:pass@host)"`
	Proxies           []string `json:"proxies,omitempty" jsonschema:"Pool of proxy URLs to rotate across"`
	ProxyRotation     string   `json:"proxy_rotation,omitempty" jsonschema:"Rotation across proxies: off, round-robin, random, or sticky-host"`
	BasicAuth         string   `json:"basic_auth,omitempty" jsonschema:"HTTP Basic Auth credentials as user:pass, for sites gated by server-level Basic Auth (e.g. a staging/acceptance environment)"`
}

// ToConfig validates and maps p onto a config.Config, returning the normalized seed URL
// (scheme defaulted, any embedded userinfo stripped into BasicAuth) to crawl.
func (p Params) ToConfig() (config.Config, string, error) {
	seed := strings.TrimSpace(p.URL)
	if seed == "" {
		return config.Config{}, "", fmt.Errorf("url is required")
	}
	if !strings.Contains(seed, "://") {
		seed = "https://" + seed
	}

	cfg := config.Default()
	if p.Depth != nil {
		cfg.Crawl.MaxDepth = *p.Depth
	}
	if p.MaxPages != nil {
		cfg.Crawl.MaxPages = *p.MaxPages
	}
	if p.Concurrency != nil {
		cfg.Crawl.Concurrency = *p.Concurrency
	}
	if strings.TrimSpace(p.MaxDuration) != "" {
		d, derr := time.ParseDuration(strings.TrimSpace(p.MaxDuration))
		if derr != nil {
			return config.Config{}, "", fmt.Errorf("max_duration %q: %w", p.MaxDuration, derr)
		}
		cfg.Crawl.MaxDuration = d
	}
	if p.Render != "" {
		cfg.Render = p.Render
	}
	if p.RespectRobots != nil {
		cfg.Crawl.RespectRobots = *p.RespectRobots
	}
	if p.Subdomains != nil {
		cfg.Crawl.AllowSubdomains = *p.Subdomains
	}
	cfg.Analyzers.Enabled = p.Analyzers
	if p.Specialized != nil {
		cfg.Analyzers.Specialized = *p.Specialized
	}
	cfg.Crawl.Include = p.Include
	cfg.Crawl.Exclude = p.Exclude
	if p.UserAgent != "" {
		cfg.Crawl.UserAgent = p.UserAgent
	}
	cfg.Crawl.UserAgents = p.UserAgents
	cfg.Crawl.UserAgentRotation = p.UserAgentRotation
	cfg.Crawl.Proxy = p.Proxy
	cfg.Crawl.Proxies = p.Proxies
	cfg.Crawl.ProxyRotation = p.ProxyRotation
	cfg.Crawl.BasicAuth = p.BasicAuth

	var user, pass string
	seed, user, pass = crawler.SanitizeSeed(seed)
	if user != "" && cfg.Crawl.BasicAuth == "" {
		cfg.Crawl.BasicAuth = user + ":" + pass
	}

	return cfg, seed, nil
}
