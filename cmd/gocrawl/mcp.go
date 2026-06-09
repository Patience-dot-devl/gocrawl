package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/mcpserver"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run gocrawl as an MCP server over stdio",
		Long:  "Starts a Model Context Protocol server on stdin/stdout exposing 'crawl' and\n'list_analyzers' tools, so agentic tools like Conductor or Claude Code can drive\ngocrawl directly.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			server := mcpserver.New(version)
			return server.Run(context.Background(), &mcp.StdioTransport{})
		},
	}
}
