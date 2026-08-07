// Package mcpserver exposes gocrawl over the Model Context Protocol so agentic tools such
// as Conductor or Claude Code can drive crawls directly. It registers two tools: "crawl"
// (run a crawl + analysis and return a structured report) and "list_analyzers".
package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/crawlrequest"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// CrawlInput is the MCP "crawl" tool input. Optional fields override the defaults. It is an
// alias for crawlrequest.Params, the mapping shared with the web API.
type CrawlInput = crawlrequest.Params

// CrawlReport mirrors report.Report for the MCP "crawl" tool, minus SiteMap. SiteMap's
// *sitemapgen.Node is self-referential (Children []*Node), and the go-sdk schema reflector
// panics on cyclic types when building the tool's output schema. Agents reason over the flat
// Issues list, not the tree, so it's dropped here; the CLI/web JSON report still includes it.
type CrawlReport struct {
	Seed         string            `json:"seed"`
	StartedAt    string            `json:"started_at"`
	FinishedAt   string            `json:"finished_at"`
	PagesCrawled int               `json:"pages_crawled"`
	Summary      report.Summary    `json:"summary"`
	Issues       []analyze.Issue   `json:"issues"`
	Notes        []string          `json:"notes,omitempty"`
	Coverage     *crawler.Coverage `json:"coverage,omitempty"`
}

// CrawlOutput is the MCP "crawl" tool output.
type CrawlOutput struct {
	Report CrawlReport `json:"report"`
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
	cfg, seed, err := in.ToConfig()
	if err != nil {
		return nil, CrawlOutput{}, err
	}
	rep, err := runner.Run(ctx, cfg, seed)
	if err != nil {
		return nil, CrawlOutput{}, err
	}
	return nil, CrawlOutput{Report: CrawlReport{
		Seed:         rep.Seed,
		StartedAt:    rep.StartedAt,
		FinishedAt:   rep.FinishedAt,
		PagesCrawled: rep.PagesCrawled,
		Summary:      rep.Summary,
		Issues:       rep.Issues,
		Notes:        rep.Notes,
		Coverage:     rep.Coverage,
	}}, nil
}

func handleListAnalyzers(_ context.Context, _ *mcp.CallToolRequest, _ ListAnalyzersInput) (*mcp.CallToolResult, ListAnalyzersOutput, error) {
	return nil, ListAnalyzersOutput{Analyzers: runner.ListAnalyzers()}, nil
}
