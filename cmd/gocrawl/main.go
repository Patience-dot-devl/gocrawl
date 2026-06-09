// Command gocrawl is a customizable website crawler for SEO and SEA audits.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "0.1.0-dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gocrawl",
		Short:         "A customizable FOSS website crawler for SEO & SEA audits",
		Long:          "gocrawl crawls a website concurrently and runs a pipeline of pluggable analyzers\n(technical SEO, redirects, broken links, robots.txt, sitemap, structured data, …),\nthen writes a JSON or CSV report. It can also run as an MCP server.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.PersistentFlags().StringP("config", "c", "", "path to a YAML config file")

	root.AddCommand(newCrawlCmd())
	root.AddCommand(newAnalyzersCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newMCPCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
