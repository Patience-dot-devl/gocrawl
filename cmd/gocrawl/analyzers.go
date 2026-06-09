package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

func newAnalyzersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyzers",
		Short: "Manage and inspect analyzers",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available analyzers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, a := range runner.ListAnalyzers() {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %s\n", a.Name, a.Description)
			}
			return nil
		},
	})
	return cmd
}
