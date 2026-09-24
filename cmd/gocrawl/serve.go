package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
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
	f.String("addr", "127.0.0.1:8080", "address to listen on; the server has no authentication, so binding to a non-loopback address exposes it to the network")
	f.String("store-dir", "", "store directory for crawl history (default: ~/.gocrawl/crawls)")
	return cmd
}

func runServe(cmd *cobra.Command, _ []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	st, err := newStore(cmd)
	if err != nil {
		return err
	}

	var opts []webserver.Option
	if !isLoopbackBind(addr) {
		// The operator asked for a reachable server; the loopback Host check would only
		// lock out the clients they just opened the door to.
		opts = append(opts, webserver.AllowAnyHost())
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: --addr %s is not loopback; the web API has no authentication and anyone who can reach it can start crawls from this machine\n", addr)
	}
	srv := newHTTPServer(addr, webserver.New(st, opts...).Handler())

	ctx := cmd.Context()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "gocrawl web UI listening on http://%s\n", displayAddr(addr)); err != nil {
		return err
	}

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

// newHTTPServer builds the listener with header/idle timeouts so a client that opens a
// connection and never finishes its request headers can't pin a goroutine forever. There is
// no WriteTimeout: report exports can be large, and the crawl itself runs detached from the
// request that started it, so no handler is legitimately slow enough to need one.
func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// isLoopbackBind reports whether a listen address only accepts local connections. An empty
// host (":8080") binds every interface.
func isLoopbackBind(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// displayAddr turns a bind address like ":8080" into a browsable "localhost:8080".
func displayAddr(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return "localhost" + addr
	}
	return addr
}
