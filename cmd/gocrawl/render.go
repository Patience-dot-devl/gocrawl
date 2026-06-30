package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/sitemapgen"
)

func newRenderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render <report.json>",
		Short: "Re-render a saved JSON report into another format without recrawling",
		Long: "Read a JSON report previously produced by `gocrawl crawl --format json` and write it\n" +
			"out in another format (HTML by default). This is the fast way to regenerate an HTML\n" +
			"report — e.g. after a gocrawl upgrade with template improvements — without paying for\n" +
			"a full recrawl. The JSON carries the site-map tree, so the HTML Site map tab is\n" +
			"reproduced too; only the optional standalone sitemap.xml side output is regenerated\n" +
			"separately via --sitemap.",
		Args: cobra.ExactArgs(1),
		RunE: runRender,
	}
	f := cmd.Flags()
	f.StringP("out", "o", "", "output file (default: stdout)")
	f.StringP("format", "f", "html", "output format: json, csv, or html")
	f.String("sitemap", "", "also write a standard sitemap.xml of the report's pages to this path")
	return cmd
}

func runRender(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("reading report %q: %w", args[0], err)
	}
	var rep report.Report
	// Lenient decode on purpose: a report written by a newer gocrawl may carry fields this
	// build doesn't know, which shouldn't block a re-render.
	if err := json.Unmarshal(data, &rep); err != nil {
		return fmt.Errorf("parsing report %q as JSON: %w", args[0], err)
	}

	f := cmd.Flags()
	format, _ := f.GetString("format")
	out, _ := f.GetString("out")
	cfg := config.Config{Output: config.OutputConfig{Format: format, Path: out}}
	if err := cfg.Validate(); err != nil {
		return err
	}

	if sitemapPath, _ := f.GetString("sitemap"); sitemapPath != "" {
		if rep.SiteMap == nil {
			return fmt.Errorf("report %q has no site map to write (was it produced by an older gocrawl?)", args[0])
		}
		if err := sitemapgen.WriteXMLFile(sitemapPath, *rep.SiteMap); err != nil {
			return fmt.Errorf("writing sitemap.xml: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Sitemap written to %s\n", sitemapPath)
	}

	return writeReport(cfg, &rep)
}
