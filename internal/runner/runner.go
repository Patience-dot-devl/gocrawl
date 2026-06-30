// Package runner wires the crawl engine, analyzer registry, and report builder into a
// single entry point used by both the CLI and the MCP server.
package runner

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/aeo"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/amp"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/botwall"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/content"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/datalayer"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/duplicates"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/geo"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/hreflang"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/httpx"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/images"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/landing"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/links"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/pagination"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/perf"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/robotscheck"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/security"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/seo"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/sitemap"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/structured"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/tracking"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/urls"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/utm"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/wordpress"
	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/render"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/sitemapgen"
)

// queryDependentAnalyzers read URL query parameters, so they produce nothing useful when
// crawl.strip_query drops the query string. Run skips them (with a note) in that mode.
var queryDependentAnalyzers = []string{"utm", "landing", "wordpress"}

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
	// Content & technical SEO breadth checks (Screaming Frog parity, tier 1).
	r.Register(images.New())
	r.Register(urls.New())
	r.Register(security.New())
	r.Register(pagination.New())
	r.Register(hreflang.New())
	r.Register(amp.New())
	r.Register(duplicates.New())
	r.Register(content.New())
	r.Register(botwall.New())
	// CMS-specific checks. WordPress security probes are active (extra fetches), so they ride
	// the same specialized flag as the opt-in AI-search heuristics.
	r.Register(wordpress.New(fetcher, wordpress.WithSecurityProbes(specialized)))
	// SEA (Search Engine Advertising) analyzers.
	r.Register(utm.New())
	r.Register(tracking.New())
	r.Register(datalayer.New())
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
	analyzers, skipped := planAnalyzers(reg, cfg.Analyzers, cfg.Crawl.StripQuery)
	issues := analyze.Run(ctx, analyzers, result)

	rep := report.Build(result, issues)
	if len(skipped) > 0 {
		rep.Notes = append(rep.Notes, fmt.Sprintf("strip_query is on, so query-dependent analyzers were skipped: %s", strings.Join(skipped, ", ")))
	}
	if result.ThrottleEvents > 0 {
		rep.Notes = append(rep.Notes, fmt.Sprintf("server returned HTTP 429/503: adaptive delay slowed the crawl %d time(s), down to %.3g req/s (disable with --adaptive-delay=false)", result.ThrottleEvents, result.FinalRate))
	}
	if !result.Coverage.Complete {
		rep.Notes = append(rep.Notes, coverageNote(result.Coverage))
	}

	// The crawled site is rendered as a tab in the HTML report (report.Build attaches it). When
	// --sitemap is set, also write it out as a standalone, machine-readable sitemap.xml.
	if cfg.Output.SitemapPath != "" && rep.SiteMap != nil {
		if err := sitemapgen.WriteXMLFile(cfg.Output.SitemapPath, *rep.SiteMap); err != nil {
			return nil, fmt.Errorf("writing sitemap.xml: %w", err)
		}
		rep.Notes = append(rep.Notes, fmt.Sprintf("sitemap.xml written to %s (%d URLs)", cfg.Output.SitemapPath, len(rep.SiteMap.Entries)))
	}
	return rep, nil
}

// coverageNote turns an incomplete-coverage result into an actionable advisory, naming the
// limit that cut the crawl short and how to lift it. This is the signal that keeps "0 broken
// links" from being misread as "no broken links" when the crawl simply didn't reach them.
func coverageNote(c crawler.Coverage) string {
	var reasons []string
	if c.PageLimitReached {
		reasons = append(reasons, fmt.Sprintf("the page limit (--max-pages %d) was reached — raise it or set 0 for unlimited", c.MaxPages))
	}
	if c.DepthLimitReached {
		reasons = append(reasons, fmt.Sprintf("the depth limit (--depth %d) was reached — raise it or set 0 for unlimited", c.MaxDepth))
	}
	why := ""
	if len(reasons) > 0 {
		why = " because " + strings.Join(reasons, ", and ")
	}
	return fmt.Sprintf("partial coverage: %d in-scope URL(s) were discovered but not crawled%s. "+
		"Page-level findings (broken links especially) may be incomplete — re-crawl with a higher limit for full coverage.",
		c.DiscoveredNotCrawled, why)
}

// planAnalyzers selects the analyzers to run for the given config, and returns the names of any
// query-dependent analyzers skipped because strip_query is on. strip_query drops query strings
// while crawling, so analyzers that read query parameters (UTM tags on links, WordPress
// ?p=/?attachment_id=/?s= URLs) would only produce misleading empty results; they are skipped
// rather than run.
func planAnalyzers(reg *analyze.Registry, ac config.AnalyzersConfig, stripQuery bool) ([]analyze.Analyzer, []string) {
	if !stripQuery {
		return reg.Select(ac.Enabled, ac.Disabled), nil
	}
	active := names(reg.Select(ac.Enabled, ac.Disabled))
	var skipped []string
	for _, name := range queryDependentAnalyzers {
		if active[name] {
			skipped = append(skipped, name)
		}
	}
	disabled := append(append([]string{}, ac.Disabled...), queryDependentAnalyzers...)
	return reg.Select(ac.Enabled, disabled), skipped
}

// names returns the set of analyzer names in the given slice.
func names(as []analyze.Analyzer) map[string]bool {
	out := make(map[string]bool, len(as))
	for _, a := range as {
		out[a.Name()] = true
	}
	return out
}
