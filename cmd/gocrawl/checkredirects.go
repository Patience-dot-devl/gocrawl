package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

func newCheckRedirectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-redirects",
		Short: "Verify a redirect-rule CSV export against a live site",
		Long: "check-redirects reads a HubSpot-format URL Redirects CSV export and checks each\n" +
			"in-scope rule against the live site: whether the source still redirects correctly,\n" +
			"whether the target is a live page, and whether both agree with the current sitemap.xml.\n" +
			"It writes the input CSV back out with appended verdict columns.",
		RunE: runCheckRedirects,
	}
	f := cmd.Flags()
	f.String("input", "", "path to the redirect-rule CSV (required)")
	f.String("domain", "", "main domain; subdomains are in-scope, other domains are skipped (required)")
	f.String("output", "", "output CSV path (default: stdout)")
	f.String("sitemap-url", "", "sitemap URL to use if the default locations (/sitemap.xml, /sitemap_index.xml) don't work")
	f.Int("concurrency", 4, "parallel fetch workers")
	f.Float64("rate", 0, "max requests per second (0 = unlimited)")
	f.Duration("timeout", 15*time.Second, "per-request timeout")
	f.String("user-agent", "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)", "User-Agent header")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("domain")
	return cmd
}

func runCheckRedirects(cmd *cobra.Command, _ []string) error {
	f := cmd.Flags()
	inputPath, _ := f.GetString("input")
	domain, _ := f.GetString("domain")
	outputPath, _ := f.GetString("output")
	sitemapURL, _ := f.GetString("sitemap-url")
	concurrency, _ := f.GetInt("concurrency")
	rateLimit, _ := f.GetFloat64("rate")
	timeout, _ := f.GetDuration("timeout")
	userAgent, _ := f.GetString("user-agent")

	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("opening input: %w", err)
	}
	defer func() { _ = file.Close() }()

	rules, err := redirectcheck.ParseCSV(file)
	if err != nil {
		return fmt.Errorf("parsing input CSV: %w", err)
	}

	fetcher := crawler.NewHTTPFetcher(crawler.Options{
		Timeout:      timeout,
		UserAgent:    userAgent,
		MaxRedirects: 10,
		MaxBodyBytes: 5 << 20,
	})

	results, err := redirectcheck.Run(cmd.Context(), rules, redirectcheck.RunOptions{
		Domain:        domain,
		SitemapURL:    sitemapURL,
		Fetcher:       fetcher,
		Concurrency:   concurrency,
		RatePerSecond: rateLimit,
	})
	if err != nil {
		return err
	}

	out := os.Stdout
	if outputPath != "" {
		outFile, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("creating output: %w", err)
		}
		defer func() { _ = outFile.Close() }()
		out = outFile
	}
	if err := redirectcheck.WriteCSV(out, rules, results); err != nil {
		return fmt.Errorf("writing output CSV: %w", err)
	}
	if outputPath != "" {
		fmt.Fprintf(os.Stderr, "Report written to %s\n", outputPath)
	}
	return nil
}
