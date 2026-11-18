package main

import (
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
	f.Duration("max-duration", 0, "budget for active crawl time (paused time excluded), e.g. 90m (0 = unlimited); on expiry the crawl stops early and still writes a partial report")
	f.String("render", "", "rendering mode: raw or headless")
	f.StringP("out", "o", "", "output file (default: stdout)")
	f.StringP("format", "f", "", "output format: json, csv, or html")
	f.String("sitemap", "", "also write a standard sitemap.xml of crawled pages to this path")
	f.String("list", "", "list mode: crawl only the URLs in this file (one per line, # for comments; - reads stdin) and follow no links")
	f.Bool("from-sitemap", false, "list mode: crawl only the URLs in the site's sitemap and follow no links")
	f.String("fail-on", "", "exit with a non-zero status, after writing the report, if any issue is at or above this severity: error, warning, or info")
	f.Bool("per-page-info", false, "list every info finding per page, instead of once per host when it repeats across most of a host's pages")
	f.StringSlice("include", nil, "only crawl URLs matching these regexes")
	f.StringSlice("exclude", nil, "skip URLs matching these regexes")
	f.String("user-agent", "", "User-Agent header")
	f.StringSlice("user-agents", nil, "pool of User-Agent strings to rotate across (supersedes --user-agent)")
	f.String("user-agent-rotation", "", "rotation across --user-agents: off, round-robin, or random (default round-robin)")
	f.String("proxy", "", "route requests through this proxy URL (http(s):// or socks5://; supports user:pass@host)")
	f.StringSlice("proxies", nil, "pool of proxy URLs to rotate across")
	f.String("proxy-rotation", "", "rotation across proxies: off, round-robin, random, or sticky-host (default round-robin)")
	f.String("basic-auth", "", "HTTP Basic Auth credentials as user:pass, for sites gated by server-level Basic Auth (e.g. a staging/acceptance environment)")
	f.String("cookie", "", "raw Cookie header sent on every request, for sites gated by an app-level session cookie (e.g. a Shopify storefront password page) rather than server-level Basic Auth")
	f.StringArray("header", nil, `extra request header as "Name: value" (repeatable), sent only to the seed host like --cookie`)
	f.String("header-file", "", `file of extra request headers, one "Name: value" per line ('#' comments allowed)`)
	f.String("header-profile", "", "named header file at ~/.gocrawl/headers/<name>.headers")
	f.String("bearer-token", "", `token sent as "Authorization: Bearer <token>", only to the seed host like --cookie (a flag value lands in shell history; GOCRAWL_CRAWL_BEARER_TOKEN doesn't)`)
	f.Bool("respect-robots", true, "obey robots.txt while crawling")
	f.Bool("subdomains", false, "follow links to subdomains of the seed")
	f.Bool("external", false, "crawl links that leave the seed host")
	f.Bool("strip-query", false, "ignore query strings, treating ?a=1 and ?a=2 as one URL (disables utm/tracking query analysis)")
	f.BoolP("verbose", "v", false, "log each fetch and rate-limit change to stderr while crawling")
	f.Bool("adaptive-delay", true, "automatically slow the crawl when the server returns HTTP 429/503")
	f.StringSlice("analyzers", nil, "only run these analyzers (comma-separated)")
	f.Bool("specialized", false, "enable opt-in specialized checks (AEO answer-lead, GEO quotable-density, WordPress security probes, Shopify /products.json probe)")
	f.Bool("security-audit", false, "enable the opt-in security audit (TLS/certificate, cookie attributes, response-header hygiene)")
	f.Bool("ignore-external-tagging", true, "ignore the utm analyzer's tagging-quality warnings for links leaving the domain")
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
	if path, _ := cmd.Flags().GetString("list"); path != "" {
		urls, err := readURLList(path, cmd.InOrStdin())
		if err != nil {
			return err
		}
		cfg.Crawl.URLs = append(cfg.Crawl.URLs, urls...)
	}
	for i, u := range cfg.Crawl.URLs {
		cfg.Crawl.URLs[i] = withScheme(u)
	}
	if seed == "" && len(cfg.Crawl.URLs) > 0 {
		// In list mode the seed only fixes the crawl's host scope: default it to the first URL.
		seed = cfg.Crawl.URLs[0]
	}

	if seed == "" {
		return fmt.Errorf("no seed URL given (pass a URL argument, set seed in config, or pass --list)")
	}
	seed = withScheme(seed)
	var user, pass string
	seed, user, pass = crawler.SanitizeSeed(seed)
	if user != "" && cfg.Crawl.BasicAuth == "" {
		cfg.Crawl.BasicAuth = user + ":" + pass
	}

	rep, err := runner.Run(cmd.Context(), cfg, seed)
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
	return failOn(cfg.Output.FailOn, rep)
}

// failOn returns an error when the report holds an issue at or above severity (error >
// warning > info), so `gocrawl crawl --fail-on error` can gate a CI pipeline. It runs after
// the report is written, so a failing build still has it. An empty severity never fails.
func failOn(severity string, rep *report.Report) error {
	levels := map[string][]string{
		"error":   {"error"},
		"warning": {"error", "warning"},
		"info":    {"error", "warning", "info"},
	}
	n := 0
	for _, sev := range levels[severity] {
		n += rep.Summary.BySeverity[sev]
	}
	if n > 0 {
		return fmt.Errorf("%d issue(s) at or above %s severity (--fail-on %s)", n, severity, severity)
	}
	return nil
}

// withScheme adds https:// to a URL given without a scheme, as a seed argument may be.
func withScheme(u string) string {
	if !strings.Contains(u, "://") {
		return "https://" + u
	}
	return u
}

// readURLList reads a list-mode URL file: one URL per line, blank lines and lines starting
// with # skipped, surrounding space trimmed. "-" reads stdin.
func readURLList(path string, stdin io.Reader) ([]string, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading --list: %w", err)
	}
	var urls []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("--list %s holds no URLs", path)
	}
	return urls, nil
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
	if f.Changed("max-duration") {
		cfg.Crawl.MaxDuration, _ = f.GetDuration("max-duration")
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
	if f.Changed("from-sitemap") {
		cfg.Crawl.FromSitemap, _ = f.GetBool("from-sitemap")
	}
	if f.Changed("fail-on") {
		cfg.Output.FailOn, _ = f.GetString("fail-on")
	}
	if f.Changed("per-page-info") {
		cfg.Output.PerPageInfo, _ = f.GetBool("per-page-info")
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
	if f.Changed("cookie") {
		cfg.Crawl.Cookie, _ = f.GetString("cookie")
	}
	if f.Changed("header") {
		// StringArray, not StringSlice: header values routinely contain commas (e.g.
		// Signature-Input), which StringSlice would split into separate entries.
		cfg.Crawl.Headers, _ = f.GetStringArray("header")
	}
	if f.Changed("header-file") {
		cfg.Crawl.HeaderFile, _ = f.GetString("header-file")
	}
	if f.Changed("header-profile") {
		cfg.Crawl.HeaderProfile, _ = f.GetString("header-profile")
	}
	if f.Changed("bearer-token") {
		cfg.Crawl.BearerToken, _ = f.GetString("bearer-token")
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
	if f.Changed("security-audit") {
		cfg.Analyzers.SecurityAudit, _ = f.GetBool("security-audit")
	}
	if f.Changed("ignore-external-tagging") {
		cfg.Analyzers.IgnoreExternalTagging, _ = f.GetBool("ignore-external-tagging")
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
