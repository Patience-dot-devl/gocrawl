package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

func newCrawlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crawl [url]",
		Short: "Crawl a site and write an SEO/SEA report",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runCrawl,
	}
	f := cmd.Flags()
	f.IntP("depth", "d", 0, "max link depth from the seed (0 = seed only)")
	f.Int("max-pages", 0, "max pages to crawl")
	f.Int("concurrency", 0, "parallel fetch workers")
	f.Float64("rate", 0, "max requests per second (0 = unlimited)")
	f.String("render", "", "rendering mode: raw or headless")
	f.StringP("out", "o", "", "output file (default: stdout)")
	f.StringP("format", "f", "", "output format: json, csv, or html")
	f.StringSlice("include", nil, "only crawl URLs matching these regexes")
	f.StringSlice("exclude", nil, "skip URLs matching these regexes")
	f.String("user-agent", "", "User-Agent header")
	f.Bool("respect-robots", true, "obey robots.txt while crawling")
	f.Bool("subdomains", false, "follow links to subdomains of the seed")
	f.Bool("external", false, "crawl links that leave the seed host")
	f.Bool("strip-query", false, "ignore query strings, treating ?a=1 and ?a=2 as one URL (disables utm/tracking query analysis)")
	f.StringSlice("analyzers", nil, "only run these analyzers (comma-separated)")
	f.Bool("specialized", false, "enable opt-in specialized checks (AEO answer-lead, GEO quotable-density, WordPress security probes)")
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

	rep, err := runner.Run(context.Background(), cfg, seed)
	if err != nil {
		return err
	}

	if err := writeReport(cfg, rep); err != nil {
		return err
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
	if f.Changed("include") {
		cfg.Crawl.Include, _ = f.GetStringSlice("include")
	}
	if f.Changed("exclude") {
		cfg.Crawl.Exclude, _ = f.GetStringSlice("exclude")
	}
	if f.Changed("user-agent") {
		cfg.Crawl.UserAgent, _ = f.GetString("user-agent")
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
	if f.Changed("analyzers") {
		cfg.Analyzers.Enabled, _ = f.GetStringSlice("analyzers")
	}
	if f.Changed("specialized") {
		cfg.Analyzers.Specialized, _ = f.GetBool("specialized")
	}
}

func writeReport(cfg config.Config, rep *report.Report) error {
	reporter := report.For(cfg.Output.Format)
	if cfg.Output.Path == "" {
		return reporter.Write(os.Stdout, rep)
	}
	if dir := filepath.Dir(cfg.Output.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating output directory %q: %w", dir, err)
		}
	}
	file, err := os.Create(cfg.Output.Path)
	if err != nil {
		return err
	}
	writeErr := reporter.Write(file, rep)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Fprintf(os.Stderr, "Report written to %s\n", cfg.Output.Path)
	return nil
}
