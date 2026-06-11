// Package mcpserver exposes gocrawl over the Model Context Protocol so agentic tools such
// as Conductor or Claude Code can drive crawls directly. It registers two tools: "crawl"
// (run a crawl + analysis and return a structured report) and "list_analyzers".
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// CrawlInput is the MCP "crawl" tool input. Optional fields override the defaults.
type CrawlInput struct {
	URL           string   `json:"url" jsonschema:"Seed URL to crawl (e.g. https://example.com)"`
	Depth         *int     `json:"depth,omitempty" jsonschema:"Maximum link depth from the seed (default 2; 0 = seed only)"`
	MaxPages      *int     `json:"max_pages,omitempty" jsonschema:"Hard cap on the number of pages crawled (default 500)"`
	Concurrency   *int     `json:"concurrency,omitempty" jsonschema:"Number of parallel fetch workers (default 4)"`
	Render        string   `json:"render,omitempty" jsonschema:"Rendering mode: 'raw' (default) or 'headless'"`
	Analyzers     []string `json:"analyzers,omitempty" jsonschema:"Subset of analyzer names to run; empty runs all"`
	Specialized   *bool    `json:"specialized,omitempty" jsonschema:"Enable opt-in specialized AI-search checks (AEO answer-lead, GEO quotable-density); off by default"`
	RespectRobots *bool    `json:"respect_robots,omitempty" jsonschema:"Obey robots.txt while crawling (default true)"`
	Subdomains    *bool    `json:"subdomains,omitempty" jsonschema:"Follow links to subdomains of the seed host"`
	Include       []string `json:"include,omitempty" jsonschema:"Only crawl URLs matching at least one of these regexes"`
	Exclude       []string `json:"exclude,omitempty" jsonschema:"Skip URLs matching any of these regexes"`
}

// CrawlOutput is the MCP "crawl" tool output: the full crawl report.
type CrawlOutput struct {
	Report *report.Report `json:"report"`
}

// ListAnalyzersInput is the (empty) input for the "list_analyzers" tool.
type ListAnalyzersInput struct{}

// ListAnalyzersOutput lists the available analyzers.
type ListAnalyzersOutput struct {
	Analyzers []runner.AnalyzerInfo `json:"analyzers"`
}

// New builds the gocrawl MCP server.
func New(version string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "gocrawl", Version: version}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "crawl",
		Description: "Crawl a website and run the SEO/SEA analyzer pipeline, returning a structured report of issues (technical SEO, redirects, broken links, robots.txt, sitemap coverage, structured data, performance).",
	}, handleCrawl)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_analyzers",
		Description: "List the available gocrawl analyzers and their descriptions.",
	}, handleListAnalyzers)

	return s
}

func handleCrawl(ctx context.Context, _ *mcp.CallToolRequest, in CrawlInput) (*mcp.CallToolResult, CrawlOutput, error) {
	seed := strings.TrimSpace(in.URL)
	if seed == "" {
		return nil, CrawlOutput{}, fmt.Errorf("url is required")
	}
	if !strings.Contains(seed, "://") {
		seed = "https://" + seed
	}

	cfg := config.Default()
	if in.Depth != nil {
		cfg.Crawl.MaxDepth = *in.Depth
	}
	if in.MaxPages != nil {
		cfg.Crawl.MaxPages = *in.MaxPages
	}
	if in.Concurrency != nil {
		cfg.Crawl.Concurrency = *in.Concurrency
	}
	if in.Render != "" {
		cfg.Render = in.Render
	}
	if in.RespectRobots != nil {
		cfg.Crawl.RespectRobots = *in.RespectRobots
	}
	if in.Subdomains != nil {
		cfg.Crawl.AllowSubdomains = *in.Subdomains
	}
	cfg.Analyzers.Enabled = in.Analyzers
	if in.Specialized != nil {
		cfg.Analyzers.Specialized = *in.Specialized
	}
	cfg.Crawl.Include = in.Include
	cfg.Crawl.Exclude = in.Exclude

	rep, err := runner.Run(ctx, cfg, seed)
	if err != nil {
		return nil, CrawlOutput{}, err
	}
	return nil, CrawlOutput{Report: rep}, nil
}

func handleListAnalyzers(_ context.Context, _ *mcp.CallToolRequest, _ ListAnalyzersInput) (*mcp.CallToolResult, ListAnalyzersOutput, error) {
	return nil, ListAnalyzersOutput{Analyzers: runner.ListAnalyzers()}, nil
}
