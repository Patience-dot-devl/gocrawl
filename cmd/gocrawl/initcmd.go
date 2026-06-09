package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Write a commented example config file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "gocrawl.yaml"
			if len(args) == 1 {
				path = args[0]
			}
			if err := config.WriteExample(path); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Wrote example config to %s\n", path)
			return err
		},
	}
}
