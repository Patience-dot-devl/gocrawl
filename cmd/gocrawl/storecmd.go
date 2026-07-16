package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/atomicfile"
	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/diff"
	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

// newStore builds a Store from the config file and an optional --store-dir override.
func newStore(cmd *cobra.Command) (*store.Store, error) {
	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	dir := cfg.Store.Dir
	if cmd.Flags().Changed("store-dir") {
		dir, _ = cmd.Flags().GetString("store-dir")
	}
	return store.New(dir), nil
}

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history [host]",
		Short: "List crawls saved in the store",
		Long: "List crawls previously saved with `gocrawl crawl --save`, newest first. Pass a host\n" +
			"(e.g. example.com) to list only that site's crawls. The IDs shown can be passed to\n" +
			"`gocrawl compare` and `gocrawl render`.",
		Args: cobra.MaximumNArgs(1),
		RunE: runHistory,
	}
	f := cmd.Flags()
	f.String("store-dir", "", "store directory (default: ~/.gocrawl/crawls)")
	f.StringP("format", "f", "text", "output format: text or json")
	return cmd
}

func runHistory(cmd *cobra.Command, args []string) error {
	st, err := newStore(cmd)
	if err != nil {
		return err
	}
	host := ""
	if len(args) == 1 {
		host = args[0]
	}
	entries, err := st.List(host)
	if err != nil {
		return err
	}

	if format, _ := cmd.Flags().GetString("format"); format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "No saved crawls in %s. Run `gocrawl crawl <url> --save` first.\n", st.Root())
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	// Intermediate writes are buffered; the real error surfaces at Flush.
	_, _ = fmt.Fprintln(tw, "ID\tFINISHED\tPAGES\tERR\tWARN\tINFO")
	for _, e := range entries {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%d\n",
			e.ID, orDash(e.FinishedAt), e.PagesCrawled,
			e.BySeverity["error"], e.BySeverity["warning"], e.BySeverity["info"])
	}
	return tw.Flush()
}

func newCompareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <base> <current>",
		Short: "Compare two crawls and report what changed",
		Long: "Diff an earlier crawl (base) against a later one (current): which issues are new,\n" +
			"which were resolved, and which persist, plus page and summary deltas.\n\n" +
			"Each argument is a crawl reference: a path to a JSON report, a stored crawl ID\n" +
			"(host/timestamp from `gocrawl history`), the word `latest`, or a bare host name\n" +
			"(that site's newest saved crawl). Examples:\n\n" +
			"  gocrawl compare before.json after.json\n" +
			"  gocrawl compare example.com/20260601T120000Z latest\n" +
			"  gocrawl compare example.com latest",
		Args: cobra.ExactArgs(2),
		RunE: runCompare,
	}
	f := cmd.Flags()
	f.String("store-dir", "", "store directory for resolving crawl IDs (default: ~/.gocrawl/crawls)")
	f.StringP("format", "f", "text", "output format: text or json")
	f.StringP("out", "o", "", "output file (default: stdout)")
	f.Bool("fail-on-new", false, "exit with a non-zero status if the current crawl has new issues (useful in CI)")
	return cmd
}

func runCompare(cmd *cobra.Command, args []string) error {
	st, err := newStore(cmd)
	if err != nil {
		return err
	}
	baseRep, _, err := st.Resolve(args[0])
	if err != nil {
		return fmt.Errorf("base crawl: %w", err)
	}
	currentRep, _, err := st.Resolve(args[1])
	if err != nil {
		return fmt.Errorf("current crawl: %w", err)
	}

	d := diff.Compare(baseRep, currentRep)
	format, _ := cmd.Flags().GetString("format")
	reporter := diff.For(format)

	out, _ := cmd.Flags().GetString("out")
	if out == "" {
		if err := reporter.Write(os.Stdout, d); err != nil {
			return err
		}
	} else {
		if err := atomicfile.Write(out, 0o644, func(w io.Writer) error {
			return reporter.Write(w, d)
		}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Diff written to %s\n", out)
	}

	if failOnNew, _ := cmd.Flags().GetBool("fail-on-new"); failOnNew && len(d.Issues.New) > 0 {
		return fmt.Errorf("%d new issue(s) introduced since the base crawl", len(d.Issues.New))
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
