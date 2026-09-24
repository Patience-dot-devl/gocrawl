package webserver

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Option configures a Server.
type Option func(*Server)

// AllowAnyHost disables the loopback Host-header check. gocrawl serve applies it when the
// operator deliberately binds to a non-loopback address, at which point the server is
// reachable from other machines by design and the Host header can legitimately be anything.
func AllowAnyHost() Option {
	return func(s *Server) { s.allowAnyHost = true }
}

// WithMaxRunning caps the number of crawls that may run at once; POST /api/crawls returns
// 429 once the cap is reached. n <= 0 leaves the default.
func WithMaxRunning(n int) Option {
	return func(s *Server) {
		if n > 0 {
			s.maxRunning = n
		}
	}
}

// DefaultMaxRunning is the crawl-concurrency cap applied when WithMaxRunning isn't given.
const DefaultMaxRunning = 4

// guard wraps the API with the browser-facing protections. The server has no authentication:
// it is meant to be driven from a browser on the same machine, so the two threats are a web
// page the operator happens to have open (which can POST to localhost cross-origin, or reach
// it via DNS rebinding) and anything else on the network when the bind address is not
// loopback. The checks:
//
//   - Host must name a loopback address unless AllowAnyHost was set. A DNS-rebinding page
//     sends its own hostname in Host, so this stops the request before any handler runs.
//   - Non-GET requests with an Origin header must have an Origin whose host matches Host.
//     Browsers always attach Origin to cross-origin POSTs, so a hostile page cannot start or
//     cancel a crawl.
//
// Content-Type enforcement for the crawl-start body lives in handleStartCrawl.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowAnyHost && !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden: Host must be a loopback address (bind to a non-loopback --addr to allow others)", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if origin := r.Header.Get("Origin"); origin != "" && !sameHost(origin, r.Host) {
				http.Error(w, "forbidden: cross-origin request", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isLoopbackHost reports whether a Host header (host or host:port) names the local machine.
func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sameHost reports whether origin's host:port equals the request's Host header.
func sameHost(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, host)
}
