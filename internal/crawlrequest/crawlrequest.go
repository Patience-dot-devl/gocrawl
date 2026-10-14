// Package crawlrequest maps a user-facing crawl request onto a config.Config. It is the one
// seam shared by every entry point that lets a caller parameterize a crawl — today the MCP
// "crawl" tool and the web API — so the mapping (seed normalization, option overrides,
// basic-auth extraction) lives in exactly one place.
package crawlrequest

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Params is a user-facing crawl request. Optional fields override config.Default().
type Params struct {
	URL                   string   `json:"url" jsonschema:"Seed URL to crawl (e.g. https://example.com)"`
	Depth                 *int     `json:"depth,omitempty" jsonschema:"Maximum link hops from the seed (default 0 = unlimited; the crawl is bounded by max_pages)"`
	MaxPages              *int     `json:"max_pages,omitempty" jsonschema:"Hard cap on the number of pages crawled (default 500)"`
	Concurrency           *int     `json:"concurrency,omitempty" jsonschema:"Number of parallel fetch workers (default 4)"`
	RatePerSecond         *float64 `json:"rate,omitempty" jsonschema:"Max requests per second (default 0 = unlimited)"`
	MaxDuration           string   `json:"max_duration,omitempty" jsonschema:"Budget for active crawl time (time spent paused excluded) as a Go duration string, e.g. '90m' (default unlimited); on expiry the crawl stops early and still returns a partial report"`
	Render                string   `json:"render,omitempty" jsonschema:"Rendering mode: 'raw' (default) or 'headless'"`
	Analyzers             []string `json:"analyzers,omitempty" jsonschema:"Subset of analyzer names to run; empty runs all"`
	Specialized           *bool    `json:"specialized,omitempty" jsonschema:"Enable opt-in specialized checks (AEO answer-lead, GEO quotable-density, WordPress security probes, Shopify /products.json probe); off by default"`
	SecurityAudit         *bool    `json:"security_audit,omitempty" jsonschema:"Enable the opt-in security audit: TLS protocol and certificate checks, Set-Cookie attribute hygiene, and response-header policy; off by default"`
	IgnoreExternalTagging *bool    `json:"ignore_external_tagging,omitempty" jsonschema:"Suppress the utm analyzer's tagging-quality warnings for links leaving the domain (default true — the site doesn't control third-party tagging)"`
	RespectRobots         *bool    `json:"respect_robots,omitempty" jsonschema:"Obey robots.txt while crawling (default true)"`
	Subdomains            *bool    `json:"subdomains,omitempty" jsonschema:"Follow links to subdomains of the seed host"`
	FollowExternal        *bool    `json:"follow_external,omitempty" jsonschema:"Follow links that leave the seed host entirely (default false)"`
	Include               []string `json:"include,omitempty" jsonschema:"Only crawl URLs matching at least one of these regexes"`
	Exclude               []string `json:"exclude,omitempty" jsonschema:"Skip URLs matching any of these regexes"`
	StripQuery            bool     `json:"strip_query,omitempty" jsonschema:"Ignore query strings, treating ?a=1 and ?a=2 as one URL; this skips the query-dependent utm/tracking analysis (default false)"`
	AdaptiveDelay         *bool    `json:"adaptive_delay,omitempty" jsonschema:"Slow the crawl automatically when the server answers HTTP 429/503, honoring Retry-After (default true)"`
	FollowNofollow        bool     `json:"follow_nofollow,omitempty" jsonschema:"Also follow links marked rel=nofollow (default false)"`
	Timeout               string   `json:"timeout,omitempty" jsonschema:"Per-request timeout as a Go duration string, e.g. '10s' (default 15s)"`

	UserAgent         string   `json:"user_agent,omitempty" jsonschema:"User-Agent header sent on every request"`
	UserAgents        []string `json:"user_agents,omitempty" jsonschema:"Pool of User-Agent strings to rotate across (supersedes user_agent)"`
	UserAgentRotation string   `json:"user_agent_rotation,omitempty" jsonschema:"Rotation across user_agents: off, round-robin, or random"`
	Proxy             string   `json:"proxy,omitempty" jsonschema:"Route requests through this proxy URL (http(s):// or socks5://; supports user:pass@host)"`
	Proxies           []string `json:"proxies,omitempty" jsonschema:"Pool of proxy URLs to rotate across"`
	ProxyRotation     string   `json:"proxy_rotation,omitempty" jsonschema:"Rotation across proxies: off, round-robin, random, or sticky-host"`
	BasicAuth         string   `json:"basic_auth,omitempty" jsonschema:"HTTP Basic Auth credentials as user:pass, for sites gated by server-level Basic Auth (e.g. a staging/acceptance environment)"`
	BearerToken       string   `json:"bearer_token,omitempty" jsonschema:"Token sent as 'Authorization: Bearer <token>', only to the seed host (plus subdomains when subdomains is set); raw render mode only. Can't be combined with basic_auth or a custom Authorization header"`
	Cookie            string   `json:"cookie,omitempty" jsonschema:"Raw Cookie header sent on every request, for sites gated by an app-level session cookie rather than server-level Basic Auth (e.g. a Shopify storefront password page) where a valid session cookie is already available"`
	Headers           []string `json:"headers,omitempty" jsonschema:"Extra request headers as 'Name: value' strings, sent only to the seed host (plus subdomains when subdomains is set); raw render mode only. Prefer header_profile for secrets so their values stay out of the request"`
	// HeaderProfile deliberately takes a profile name rather than a file path: a path would let
	// any MCP or web API caller make gocrawl read an arbitrary local file and send its
	// "key: value" lines to a host of the caller's choosing.
	HeaderProfile string `json:"header_profile,omitempty" jsonschema:"Name of a header profile: a file of 'Name: value' lines at ~/.gocrawl/headers/<name>.headers, sent like headers. Use this for secrets such as Shopify crawler access signatures, so the values never pass through the request"`
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
	// crawler.SanitizeSeed leaves a seed it can't parse unchanged, userinfo included, and the
	// *url.Error from the first fetch would quote it. Refuse it here with a fixed message.
	if _, perr := url.Parse(seed); perr != nil {
		return config.Config{}, "", errors.New("url is not a valid web address (check for stray characters; the value is not shown)")
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
	if p.RatePerSecond != nil {
		cfg.Crawl.RatePerSecond = *p.RatePerSecond
	}
	if strings.TrimSpace(p.MaxDuration) != "" {
		d, derr := time.ParseDuration(strings.TrimSpace(p.MaxDuration))
		if derr != nil {
			return config.Config{}, "", fmt.Errorf("max_duration %q: %w", p.MaxDuration, derr)
		}
		cfg.Crawl.MaxDuration = d
	}
	if t := strings.TrimSpace(p.Timeout); t != "" {
		d, derr := time.ParseDuration(t)
		if derr != nil {
			return config.Config{}, "", fmt.Errorf("timeout %q: %w", p.Timeout, derr)
		}
		if d <= 0 {
			return config.Config{}, "", fmt.Errorf("timeout %q: must be greater than zero", p.Timeout)
		}
		cfg.Crawl.Timeout = d
	}
	cfg.Crawl.StripQuery = p.StripQuery
	cfg.Crawl.FollowNofollow = p.FollowNofollow
	if p.AdaptiveDelay != nil {
		cfg.Crawl.AdaptiveDelay = *p.AdaptiveDelay
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
	if p.FollowExternal != nil {
		cfg.Crawl.FollowExternal = *p.FollowExternal
	}
	cfg.Analyzers.Enabled = p.Analyzers
	if p.Specialized != nil {
		cfg.Analyzers.Specialized = *p.Specialized
	}
	if p.SecurityAudit != nil {
		cfg.Analyzers.SecurityAudit = *p.SecurityAudit
	}
	if p.IgnoreExternalTagging != nil {
		cfg.Analyzers.IgnoreExternalTagging = *p.IgnoreExternalTagging
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
	cfg.Crawl.BearerToken = strings.TrimSpace(p.BearerToken)
	cfg.Crawl.Cookie = p.Cookie
	cfg.Crawl.Headers = p.Headers
	cfg.Crawl.HeaderProfile = p.HeaderProfile

	var user, pass string
	seed, user, pass = crawler.SanitizeSeed(seed)
	if user != "" && cfg.Crawl.BasicAuth == "" {
		cfg.Crawl.BasicAuth = user + ":" + pass
	}

	return cfg, seed, nil
}
