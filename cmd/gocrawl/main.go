// Command gocrawl is a customizable website crawler for SEO and SEA audits.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "0.3.0"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gocrawl",
		Short:         "A customizable FOSS website crawler for SEO & SEA audits",
		Long:          "gocrawl crawls a website concurrently and runs a pipeline of pluggable analyzers\n(technical SEO, redirects, broken links, robots.txt, sitemap, structured data, …),\nthen writes a JSON or CSV report. It can also run as an MCP server.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
		// A bare `gocrawl` invocation on an interactive terminal launches the option menu;
		// otherwise (piped/CI) it falls back to printing help.
		RunE: func(cmd *cobra.Command, _ []string) error {
			if term.IsTerminal(int(os.Stdin.Fd())) {
				return runInteractive(cmd)
			}
			return cmd.Help()
		},
	}
	root.PersistentFlags().StringP("config", "c", "", "path to a YAML config file")
	// Pre-fills the User-Agent field of the interactive menu, e.g. when a site allow-lists a
	// specific UA to exempt the crawler from a CAPTCHA. `gocrawl crawl` has its own --user-agent.
	root.Flags().String("user-agent", "", "User-Agent for the interactive crawl (pre-fills the menu)")

	root.AddCommand(newCrawlCmd())
	root.AddCommand(newAnalyzersCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newRenderCmd())
	root.AddCommand(newHistoryCmd())
	root.AddCommand(newCompareCmd())
	root.AddCommand(newMCPCmd())
	root.AddCommand(newPathCmd())
	root.AddCommand(newCheckRedirectsCmd())
	return root
}

func main() {
	// A first Ctrl-C cancels the context passed down to a running crawl, which stops it
	// gracefully and still writes whatever was fetched so far as a partial report (see
	// crawler.Coverage.Interrupted) instead of the OS just killing the process and losing
	// everything. NotifyContext stops re-intercepting after the first signal, so a second
	// Ctrl-C falls through to the default terminate-immediately behavior if the crawl is
	// slow to unwind.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
