// Package config defines gocrawl's layered configuration (defaults -> YAML file -> env ->
// flags) and maps it onto crawler.Options.
package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Config is the full gocrawl configuration.
type Config struct {
	Seed      string          `mapstructure:"seed"`
	Render    string          `mapstructure:"render"`
	Crawl     CrawlConfig     `mapstructure:"crawl"`
	Output    OutputConfig    `mapstructure:"output"`
	Analyzers AnalyzersConfig `mapstructure:"analyzers"`
}

// CrawlConfig controls crawl scope and politeness.
type CrawlConfig struct {
	MaxDepth        int           `mapstructure:"max_depth"`
	MaxPages        int           `mapstructure:"max_pages"`
	Concurrency     int           `mapstructure:"concurrency"`
	RatePerSecond   float64       `mapstructure:"rate_per_second"`
	UserAgent       string        `mapstructure:"user_agent"`
	Timeout         time.Duration `mapstructure:"timeout"`
	MaxBodyBytes    int64         `mapstructure:"max_body_bytes"`
	RespectRobots   bool          `mapstructure:"respect_robots"`
	AllowSubdomains bool          `mapstructure:"allow_subdomains"`
	FollowExternal  bool          `mapstructure:"follow_external"`
	FollowNofollow  bool          `mapstructure:"follow_nofollow"`
	Include         []string      `mapstructure:"include"`
	Exclude         []string      `mapstructure:"exclude"`
}

// OutputConfig controls report output.
type OutputConfig struct {
	Format string `mapstructure:"format"`
	Path   string `mapstructure:"path"`
}

// AnalyzersConfig selects which analyzers run.
type AnalyzersConfig struct {
	Enabled  []string `mapstructure:"enabled"`
	Disabled []string `mapstructure:"disabled"`
	// Specialized turns on opt-in checks that are off by default: the lower-confidence
	// AI-search heuristics (AEO direct-answer-lead, GEO quotable-density) and the WordPress
	// analyzer's active security-endpoint probes. They only fire when this is set.
	Specialized bool `mapstructure:"specialized"`
}

// Default returns the built-in default configuration.
func Default() Config {
	o := crawler.DefaultOptions()
	return Config{
		Render: "raw",
		Crawl: CrawlConfig{
			MaxDepth:      o.MaxDepth,
			MaxPages:      o.MaxPages,
			Concurrency:   o.Concurrency,
			RatePerSecond: o.RatePerSecond,
			UserAgent:     o.UserAgent,
			Timeout:       o.Timeout,
			MaxBodyBytes:  o.MaxBodyBytes,
			RespectRobots: o.RespectRobots,
		},
		Output: OutputConfig{Format: "json"},
	}
}

// Load reads configuration from the optional YAML file at path (empty = none), overlaid on
// defaults, then GOCRAWL_* environment variables.
func Load(path string) (Config, error) {
	v := viper.New()
	setDefaults(v)
	v.SetEnvPrefix("GOCRAWL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("reading config %q: %w", path, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	d := Default()
	v.SetDefault("render", d.Render)
	v.SetDefault("output.format", d.Output.Format)
	v.SetDefault("crawl.max_depth", d.Crawl.MaxDepth)
	v.SetDefault("crawl.max_pages", d.Crawl.MaxPages)
	v.SetDefault("crawl.concurrency", d.Crawl.Concurrency)
	v.SetDefault("crawl.rate_per_second", d.Crawl.RatePerSecond)
	v.SetDefault("crawl.user_agent", d.Crawl.UserAgent)
	v.SetDefault("crawl.timeout", d.Crawl.Timeout)
	v.SetDefault("crawl.max_body_bytes", d.Crawl.MaxBodyBytes)
	v.SetDefault("crawl.respect_robots", d.Crawl.RespectRobots)
}

// Validate checks for obviously invalid settings.
func (c Config) Validate() error {
	switch c.Render {
	case "", "raw", "headless":
	default:
		return fmt.Errorf("invalid render mode %q (want raw or headless)", c.Render)
	}
	switch c.Output.Format {
	case "", "json", "csv", "html":
	default:
		return fmt.Errorf("invalid output format %q (want json, csv, or html)", c.Output.Format)
	}
	return nil
}

// ToOptions compiles the crawl config into crawler.Options, compiling include/exclude
// regexes.
func (c Config) ToOptions() (crawler.Options, error) {
	o := crawler.DefaultOptions()
	o.MaxDepth = c.Crawl.MaxDepth
	o.MaxPages = c.Crawl.MaxPages
	o.Concurrency = c.Crawl.Concurrency
	o.RatePerSecond = c.Crawl.RatePerSecond
	if c.Crawl.UserAgent != "" {
		o.UserAgent = c.Crawl.UserAgent
	}
	if c.Crawl.Timeout > 0 {
		o.Timeout = c.Crawl.Timeout
	}
	if c.Crawl.MaxBodyBytes > 0 {
		o.MaxBodyBytes = c.Crawl.MaxBodyBytes
	}
	o.RespectRobots = c.Crawl.RespectRobots
	o.AllowSubdomains = c.Crawl.AllowSubdomains
	o.FollowExternal = c.Crawl.FollowExternal
	o.FollowNofollow = c.Crawl.FollowNofollow

	inc, err := compile(c.Crawl.Include)
	if err != nil {
		return o, fmt.Errorf("include: %w", err)
	}
	exc, err := compile(c.Crawl.Exclude)
	if err != nil {
		return o, fmt.Errorf("exclude: %w", err)
	}
	o.Include = inc
	o.Exclude = exc
	return o, nil
}

func compile(patterns []string) ([]*regexp.Regexp, error) {
	var out []*regexp.Regexp
	for _, p := range patterns {
		if strings.TrimSpace(p) == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", p, err)
		}
		out = append(out, re)
	}
	return out, nil
}
