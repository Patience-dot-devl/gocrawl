package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/webserver"
)

// shutdownGrace bounds how long `gocrawl serve` waits for in-flight HTTP requests to finish
// on Ctrl-C before forcing the listener closed.
const shutdownGrace = 10 * time.Second

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run gocrawl as a web application (API + browser UI)",
		Long: "Starts an HTTP server exposing a REST API to start and inspect crawls, and serves\n" +
			"the built-in web UI for driving gocrawl from a browser at the same address.",
		Args: cobra.NoArgs,
		RunE: runServe,
	}
	f := cmd.Flags()
	f.String("addr", ":8080", "address to listen on")
	f.String("store-dir", "", "store directory for crawl history (default: ~/.gocrawl/crawls)")
	return cmd
}

func runServe(cmd *cobra.Command, _ []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	st, err := newStore(cmd)
	if err != nil {
		return err
	}

	srv := &http.Server{Addr: addr, Handler: webserver.New(st).Handler()}

	ctx := cmd.Context()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	fmt.Fprintf(cmd.OutOrStdout(), "gocrawl web UI listening on http://%s\n", displayAddr(addr))

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// displayAddr turns a bind address like ":8080" into a browsable "localhost:8080".
func displayAddr(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return "localhost" + addr
	}
	return addr
}
