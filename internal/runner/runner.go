// Package runner wires the crawl engine, analyzer registry, and report builder into a
// single entry point used by both the CLI and the MCP server.
package runner

import (
	"context"
	"io"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/aeo"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/geo"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/httpx"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/landing"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/links"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/perf"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/robotscheck"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/seo"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/sitemap"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/structured"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/tracking"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/utm"
	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/render"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// BuildRegistry constructs the default analyzer registry. The fetcher is used by analyzers
// that retrieve additional resources (e.g. the sitemap analyzer). When specialized is true,
// the opt-in, lower-confidence AI-search heuristics (AEO direct-answer-lead, GEO
// quotable-density) are turned on; otherwise they stay silent.
func BuildRegistry(fetcher crawler.Fetcher, specialized bool) *analyze.Registry {
	r := analyze.NewRegistry()
	r.Register(seo.New())
	r.Register(httpx.New())
	r.Register(links.New())
	r.Register(robotscheck.New())
	r.Register(sitemap.New(fetcher))
	r.Register(structured.New())
	r.Register(perf.New())
	// SEA (Search Engine Advertising) analyzers.
	r.Register(utm.New())
	r.Register(tracking.New())
	r.Register(landing.New())
	// AI-search analyzers: Answer Engine and Generative Engine Optimization.
	r.Register(aeo.New(aeo.WithAnswerLead(specialized)))
	r.Register(geo.New(fetcher, geo.WithQuotableDensity(specialized)))
	return r
}

// AnalyzerInfo describes one registered analyzer.
type AnalyzerInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListAnalyzers returns metadata for every registered analyzer.
func ListAnalyzers() []AnalyzerInfo {
	reg := BuildRegistry(crawler.NewHTTPFetcher(crawler.DefaultOptions()), false)
	var out []AnalyzerInfo
	for _, a := range reg.All() {
		out = append(out, AnalyzerInfo{Name: a.Name(), Description: a.Description()})
	}
	return out
}

// newFetcher returns the page fetcher for the configured render mode. Headless mode
// may fail if no Chromium binary is available.
func newFetcher(cfg config.Config, opts crawler.Options) (crawler.Fetcher, error) {
	if cfg.Render == "headless" {
		return render.NewHeadlessFetcher(opts)
	}
	return crawler.NewHTTPFetcher(opts), nil
}

// Run performs a full crawl + analysis for the given config and seed, returning a Report.
// The seed argument overrides cfg.Seed when non-empty.
func Run(ctx context.Context, cfg config.Config, seed string) (*report.Report, error) {
	if seed == "" {
		seed = cfg.Seed
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	opts, err := cfg.ToOptions()
	if err != nil {
		return nil, err
	}

	fetcher, err := newFetcher(cfg, opts)
	if err != nil {
		return nil, err
	}
	if c, ok := fetcher.(io.Closer); ok {
		defer func() { _ = c.Close() }()
	}
	engine := crawler.New(opts, fetcher)

	result, err := engine.Crawl(ctx, seed)
	if err != nil {
		return nil, err
	}

	// Sitemap analyzer fetches with a raw fetcher regardless of render mode.
	reg := BuildRegistry(crawler.NewHTTPFetcher(opts), cfg.Analyzers.Specialized)
	analyzers := reg.Select(cfg.Analyzers.Enabled, cfg.Analyzers.Disabled)
	issues := analyze.Run(ctx, analyzers, result)

	return report.Build(result, issues), nil
}
