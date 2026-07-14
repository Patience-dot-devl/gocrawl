package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/atomicfile"
	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

func newCrawlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crawl [url]",
		Short: "Crawl a site and write an SEO/SEA report",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runCrawl,
	}
	f := cmd.Flags()
	f.IntP("depth", "d", 0, "max link hops from the seed (0 = unlimited; the crawl is bounded by --max-pages)")
	f.Int("max-pages", 0, "max pages to crawl")
	f.Int("concurrency", 0, "parallel fetch workers")
	f.Float64("rate", 0, "max requests per second (0 = unlimited)")
	f.String("render", "", "rendering mode: raw or headless")
	f.StringP("out", "o", "", "output file (default: stdout)")
	f.StringP("format", "f", "", "output format: json, csv, or html")
	f.String("sitemap", "", "also write a standard sitemap.xml of crawled pages to this path")
	f.StringSlice("include", nil, "only crawl URLs matching these regexes")
	f.StringSlice("exclude", nil, "skip URLs matching these regexes")
	f.String("user-agent", "", "User-Agent header")
	f.StringSlice("user-agents", nil, "pool of User-Agent strings to rotate across (supersedes --user-agent)")
	f.String("user-agent-rotation", "", "rotation across --user-agents: off, round-robin, or random (default round-robin)")
	f.String("proxy", "", "route requests through this proxy URL (http(s):// or socks5://; supports user:pass@host)")
	f.StringSlice("proxies", nil, "pool of proxy URLs to rotate across")
	f.String("proxy-rotation", "", "rotation across proxies: off, round-robin, random, or sticky-host (default round-robin)")
	f.String("basic-auth", "", "HTTP Basic Auth credentials as user:pass, for sites gated by server-level Basic Auth (e.g. a staging/acceptance environment)")
	f.Bool("respect-robots", true, "obey robots.txt while crawling")
	f.Bool("subdomains", false, "follow links to subdomains of the seed")
	f.Bool("external", false, "crawl links that leave the seed host")
	f.Bool("strip-query", false, "ignore query strings, treating ?a=1 and ?a=2 as one URL (disables utm/tracking query analysis)")
	f.BoolP("verbose", "v", false, "log each fetch and rate-limit change to stderr while crawling")
	f.Bool("adaptive-delay", true, "automatically slow the crawl when the server returns HTTP 429/503")
	f.StringSlice("analyzers", nil, "only run these analyzers (comma-separated)")
	f.Bool("specialized", false, "enable opt-in specialized checks (AEO answer-lead, GEO quotable-density, WordPress security probes)")
	f.Bool("save", false, "also save the crawl to the store for later `gocrawl history` / `gocrawl compare`")
	f.String("store-dir", "", "store directory for --save (default: ~/.gocrawl/crawls)")
	return cmd
}

func runCrawl(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	seed := cfg.Seed
	if len(args) == 1 {
		seed = args[0]
	}
	applyFlagOverrides(cmd, &cfg)

	if seed == "" {
		return fmt.Errorf("no seed URL given (pass a URL argument or set seed in config)")
	}
	if !strings.Contains(seed, "://") {
		seed = "https://" + seed
	}
	var user, pass string
	seed, user, pass = crawler.SanitizeSeed(seed)
	if user != "" && cfg.Crawl.BasicAuth == "" {
		cfg.Crawl.BasicAuth = user + ":" + pass
	}

	rep, err := runner.Run(context.Background(), cfg, seed)
	if err != nil {
		return err
	}

	if err := writeReport(cfg, rep); err != nil {
		return err
	}
	if save, _ := cmd.Flags().GetBool("save"); save {
		id, err := store.New(cfg.Store.Dir).Save(rep)
		if err != nil {
			return fmt.Errorf("saving crawl to store: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Saved crawl %s\n", id)
	}
	for _, note := range rep.Notes {
		fmt.Fprintln(os.Stderr, "note:", note)
	}
	for _, line := range rep.SummaryLines() {
		fmt.Fprintln(os.Stderr, line)
	}
	return nil
}

func applyFlagOverrides(cmd *cobra.Command, cfg *config.Config) {
	f := cmd.Flags()
	if f.Changed("depth") {
		cfg.Crawl.MaxDepth, _ = f.GetInt("depth")
	}
	if f.Changed("max-pages") {
		cfg.Crawl.MaxPages, _ = f.GetInt("max-pages")
	}
	if f.Changed("concurrency") {
		cfg.Crawl.Concurrency, _ = f.GetInt("concurrency")
	}
	if f.Changed("rate") {
		cfg.Crawl.RatePerSecond, _ = f.GetFloat64("rate")
	}
	if f.Changed("render") {
		cfg.Render, _ = f.GetString("render")
	}
	if f.Changed("out") {
		cfg.Output.Path, _ = f.GetString("out")
	}
	if f.Changed("format") {
		cfg.Output.Format, _ = f.GetString("format")
	}
	if f.Changed("sitemap") {
		cfg.Output.SitemapPath, _ = f.GetString("sitemap")
	}
	if f.Changed("include") {
		cfg.Crawl.Include, _ = f.GetStringSlice("include")
	}
	if f.Changed("exclude") {
		cfg.Crawl.Exclude, _ = f.GetStringSlice("exclude")
	}
	if f.Changed("user-agent") {
		cfg.Crawl.UserAgent, _ = f.GetString("user-agent")
	}
	if f.Changed("user-agents") {
		cfg.Crawl.UserAgents, _ = f.GetStringSlice("user-agents")
	}
	if f.Changed("user-agent-rotation") {
		cfg.Crawl.UserAgentRotation, _ = f.GetString("user-agent-rotation")
	}
	if f.Changed("proxy") {
		cfg.Crawl.Proxy, _ = f.GetString("proxy")
	}
	if f.Changed("proxies") {
		cfg.Crawl.Proxies, _ = f.GetStringSlice("proxies")
	}
	if f.Changed("proxy-rotation") {
		cfg.Crawl.ProxyRotation, _ = f.GetString("proxy-rotation")
	}
	if f.Changed("basic-auth") {
		cfg.Crawl.BasicAuth, _ = f.GetString("basic-auth")
	}
	if f.Changed("respect-robots") {
		cfg.Crawl.RespectRobots, _ = f.GetBool("respect-robots")
	}
	if f.Changed("subdomains") {
		cfg.Crawl.AllowSubdomains, _ = f.GetBool("subdomains")
	}
	if f.Changed("external") {
		cfg.Crawl.FollowExternal, _ = f.GetBool("external")
	}
	if f.Changed("strip-query") {
		cfg.Crawl.StripQuery, _ = f.GetBool("strip-query")
	}
	if f.Changed("verbose") {
		cfg.Crawl.Verbose, _ = f.GetBool("verbose")
	}
	if f.Changed("adaptive-delay") {
		cfg.Crawl.AdaptiveDelay, _ = f.GetBool("adaptive-delay")
	}
	if f.Changed("analyzers") {
		cfg.Analyzers.Enabled, _ = f.GetStringSlice("analyzers")
	}
	if f.Changed("specialized") {
		cfg.Analyzers.Specialized, _ = f.GetBool("specialized")
	}
	if f.Changed("store-dir") {
		cfg.Store.Dir, _ = f.GetString("store-dir")
	}
}

func writeReport(cfg config.Config, rep *report.Report) error {
	reporter := report.For(cfg.Output.Format)
	if cfg.Output.Path == "" {
		return reporter.Write(os.Stdout, rep)
	}
	if err := atomicfile.Write(cfg.Output.Path, 0o644, func(w io.Writer) error {
		return reporter.Write(w, rep)
	}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Report written to %s\n", cfg.Output.Path)
	return nil
}
