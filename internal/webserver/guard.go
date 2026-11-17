package webserver

import (
	"crypto/subtle"
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

// WithAuthToken requires token on every request. A client presents it as
// "Authorization: Bearer <token>", or a browser opens any page once with ?token=<token>, which
// sets an HttpOnly session cookie and redirects to the same URL without it. gocrawl serve
// applies it whenever the server is reachable from other machines. An empty token is ignored.
func WithAuthToken(token string) Option {
	return func(s *Server) { s.authToken = token }
}

// AuthCookie is the session cookie that carries the auth token for browser clients.
const AuthCookie = "gocrawl_auth"

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

// guard wraps the API with the browser-facing protections. On loopback the server has no
// login: it is meant to be driven from a browser on the same machine, so the two threats are
// a web page the operator happens to have open (which can POST to localhost cross-origin, or
// reach it via DNS rebinding) and anything else on the network when the bind address is not
// loopback, which gocrawl serve answers with an auth token. The checks:
//
//   - Host must name a loopback address unless AllowAnyHost was set. A DNS-rebinding page
//     sends its own hostname in Host, so this stops the request before any handler runs.
//   - Non-GET requests with an Origin header must have an Origin whose host matches Host.
//     Browsers always attach Origin to cross-origin POSTs, so a hostile page cannot start or
//     cancel a crawl.
//   - With WithAuthToken, every request must carry the token (see authorized). This is what
//     protects a server bound to a non-loopback address, where the first check is off.
//
// Content-Type enforcement for JSON bodies lives in requireJSON (request.go).
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowAnyHost && !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden: Host must be a loopback address (bind to a non-loopback --addr to allow others)", http.StatusForbidden)
			return
		}
		if s.authToken != "" && !s.authorized(w, r) {
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

// authorized checks the auth token and reports whether the request may proceed. When it
// returns false it has already written the response: a redirect that trades a valid ?token=
// for the session cookie, or 401.
func (s *Server) authorized(w http.ResponseWriter, r *http.Request) bool {
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && s.tokenMatches(bearer) {
		return true
	}
	if c, err := r.Cookie(AuthCookie); err == nil && s.tokenMatches(c.Value) {
		return true
	}
	q := r.URL.Query()
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.tokenMatches(q.Get("token")) {
		http.SetCookie(w, &http.Cookie{
			Name:     AuthCookie,
			Value:    s.authToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
		})
		// Drop the token from the address bar, history and any Referer the page sends.
		q.Del("token")
		clean := *r.URL
		clean.RawQuery = q.Encode()
		http.Redirect(w, r, clean.RequestURI(), http.StatusSeeOther)
		return false
	}
	w.Header().Set("WWW-Authenticate", `Bearer realm="gocrawl"`)
	http.Error(w, "unauthorized: open the URL with ?token=<token> that gocrawl serve printed at startup, or send Authorization: Bearer <token>", http.StatusUnauthorized)
	return false
}

func (s *Server) tokenMatches(got string) bool {
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(s.authToken)) == 1
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
