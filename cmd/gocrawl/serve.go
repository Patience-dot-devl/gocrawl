package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	f.String("addr", "127.0.0.1:8080", "address to listen on; on a non-loopback address the server requires an auth token")
	f.String("auth-token", "", "token every request must carry (default: $GOCRAWL_AUTH_TOKEN; generated at startup when --addr is not loopback and none is set)")
	f.Bool("no-auth", false, "serve a non-loopback --addr without a token, e.g. behind a reverse proxy that authenticates; anyone who can reach the server can use it")
	f.String("store-dir", "", "store directory for crawl history (default: ~/.gocrawl/crawls)")
	return cmd
}

func runServe(cmd *cobra.Command, _ []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	st, err := newStore(cmd)
	if err != nil {
		return err
	}

	// Browser profiles of login windows an earlier, killed server never cleaned up.
	go webserver.SweepStaleLoginProfiles(os.TempDir(), webserver.StaleLoginProfileAge)

	opts := []webserver.Option{webserver.WithGUISettingsPath(webserver.DefaultGUISettingsPath())}
	token, err := serveAuthToken(cmd, addr)
	if err != nil {
		return err
	}
	if token != "" {
		opts = append(opts, webserver.WithAuthToken(token))
	}
	if !isLoopbackBind(addr) {
		// The operator asked for a reachable server; the loopback Host check would only
		// lock out the clients they just opened the door to. The token protects it instead.
		opts = append(opts, webserver.AllowAnyHost())
		if token == "" {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: --addr %s is not loopback and --no-auth is set; anyone who can reach it can start crawls from this machine and read the contents of your header profiles\n", addr)
		}
	}
	web := webserver.New(st, opts...)
	// Closes login browser windows (and deletes their temporary profiles). Deferred, so it runs
	// after the HTTP server below has shut down and no request can open a new window.
	defer web.Close()
	srv := newHTTPServer(addr, web.Handler())

	// Ctrl-C (already in cmd.Context) or SIGTERM, e.g. from a service manager or the desktop
	// shell, shuts down gracefully.
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "gocrawl web UI listening on http://%s\n", displayAddr(addr)); err != nil {
		return err
	}
	if token != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "open http://%s/?token=%s (API clients: Authorization: Bearer <token>)\n", displayAddr(addr), token); err != nil {
			return err
		}
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
		err := srv.Shutdown(shutdownCtx)
		if err != nil {
			// Grace expired with requests still running: drop them so none can outlive
			// web.Close and open a login window.
			_ = srv.Close()
		}
		return err
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

// serveAuthToken decides the token the server requires: --auth-token, else
// $GOCRAWL_AUTH_TOKEN, else a random one when addr is reachable from other machines. A
// loopback server needs none (the Host and Origin checks protect it), and --no-auth turns
// the requirement off. It returns "" when no token applies.
func serveAuthToken(cmd *cobra.Command, addr string) (string, error) {
	token, _ := cmd.Flags().GetString("auth-token")
	if token == "" {
		token = os.Getenv("GOCRAWL_AUTH_TOKEN")
	}
	if noAuth, _ := cmd.Flags().GetBool("no-auth"); noAuth {
		if token != "" {
			return "", errors.New("--no-auth can't be combined with --auth-token or GOCRAWL_AUTH_TOKEN")
		}
		return "", nil
	}
	if token != "" || isLoopbackBind(addr) {
		return token, nil
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating auth token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// displayAddr turns a bind address into one a browser on this machine can open: an
// unspecified host (":8080", "0.0.0.0:8080", "[::]:8080") becomes "localhost:8080".
func displayAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		return net.JoinHostPort("localhost", port)
	}
	return addr
}
