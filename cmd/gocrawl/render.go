package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/sitemapgen"
)

func newRenderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render <report.json | crawl-id | latest | host>",
		Short: "Re-render a saved report into another format without recrawling",
		Long: "Read a report previously produced by `gocrawl crawl --format json` (or `--save`d to\n" +
			"the store) and write it out in another format (HTML by default). This is the fast way\n" +
			"to regenerate an HTML report — e.g. after a gocrawl upgrade with template improvements\n" +
			"— without paying for a full recrawl. The JSON carries the site-map tree, so the HTML\n" +
			"Site map tab is reproduced too; only the optional standalone sitemap.xml side output is\n" +
			"regenerated separately via --sitemap.\n\n" +
			"The argument is a crawl reference: a path to a JSON report, a stored crawl ID\n" +
			"(host/timestamp from `gocrawl history`), the word `latest`, or a bare host name\n" +
			"(that site's newest saved crawl). Examples:\n\n" +
			"  gocrawl render report.json\n" +
			"  gocrawl render example.com/20260601T120000Z\n" +
			"  gocrawl render latest",
		Args: cobra.ExactArgs(1),
		RunE: runRender,
	}
	f := cmd.Flags()
	f.StringP("out", "o", "", "output file (default: stdout)")
	f.StringP("format", "f", "html", "output format: json, csv, or html")
	f.String("sitemap", "", "also write a standard sitemap.xml of the report's pages to this path")
	f.String("store-dir", "", "store directory for resolving a crawl ID (default: ~/.gocrawl/crawls)")
	return cmd
}

func runRender(cmd *cobra.Command, args []string) error {
	st, err := newStore(cmd)
	if err != nil {
		return err
	}
	rep, _, err := st.Resolve(args[0])
	if err != nil {
		return fmt.Errorf("resolving %q: %w", args[0], err)
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

	return writeReport(cfg, rep)
}
