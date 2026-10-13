package webserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpguts"
	"golang.org/x/net/idna"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/render"
)

// loginBrowser opens a browser window for a person to log in to a site by hand. ctx bounds
// only the launch and first navigation; the window lives until Close, the person closes it, or
// the process exits. Tests use a fake; production uses chromedpLoginBrowser.
type loginBrowser interface {
	Open(ctx context.Context, url string) (loginWindow, error)
}

// loginWindow is one open login window. Cookies returns the whole cookie jar, values and
// HttpOnly cookies included, with CDP's domain spelling (leading dot = domain cookie). Done is
// closed when the window goes away, by Close or by the person. Close is idempotent and
// removes the window's throwaway profile.
type loginWindow interface {
	Cookies(ctx context.Context) ([]*http.Cookie, error)
	Done() <-chan struct{}
	Close() error
}

type loginState string

const (
	loginWaiting  loginState = "waiting"
	loginCaptured loginState = "captured"
	loginClosed   loginState = "closed"
	loginExpired  loginState = "expired"
)

const (
	// loginSessionTTL ends every session an hour after it starts, captured or not.
	loginSessionTTL = time.Hour
	// maxOpenLoginWindows caps browser windows waiting for a login at once (429 beyond).
	maxOpenLoginWindows = 2
)

var (
	errLoginNotFound = errors.New("no login session with that id")
	errLoginLimit    = fmt.Errorf("%d login windows are already open; finish or cancel one first", maxOpenLoginWindows)
	errLoginShutdown = errors.New("the server is shutting down")
)

// loginStateError is a request the session's state doesn't allow (HTTP 409).
type loginStateError struct{ msg string }

func (e *loginStateError) Error() string { return e.msg }

func loginStateErr(st loginState) error {
	switch st {
	case loginCaptured:
		return &loginStateError{"this login session was already captured"}
	case loginClosed:
		return &loginStateError{"the browser window was closed before the login was captured; start a new login"}
	case loginExpired:
		return &loginStateError{"this login session expired; start a new login"}
	default:
		return &loginStateError{"the login was not captured yet; log in in the browser window, then press I'm logged in"}
	}
}

// loginSession is one browser login. Cookies live only here, in memory, and are only ever
// returned as a Cookie header for a crawl of the same host.
type loginSession struct {
	id      string
	host    string
	state   loginState
	window  loginWindow    // set while waiting; nil before Open returns and once ended
	cookies []*http.Cookie // set once captured; dropped on expiry
	timer   *time.Timer
	endedAt time.Time
}

func (s *loginSession) end() {
	s.endedAt = time.Now()
	if s.timer != nil {
		s.timer.Stop()
	}
}

// loginView is the API representation of a session: never a cookie value.
type loginView struct {
	ID          string   `json:"id"`
	State       string   `json:"state"`
	Host        string   `json:"host"`
	CookieNames []string `json:"cookie_names"`
	CookieCount int      `json:"cookie_count"`
	ExpiresAt   string   `json:"expires_at,omitempty"` // earliest cookie expiry; omitted when all are session cookies
}

func (s *loginSession) view() loginView {
	v := loginView{ID: s.id, State: string(s.state), Host: s.host, CookieNames: []string{}, CookieCount: len(s.cookies)}
	seen := map[string]bool{}
	var earliest time.Time
	for _, c := range s.cookies {
		if !seen[c.Name] {
			seen[c.Name] = true
			v.CookieNames = append(v.CookieNames, c.Name)
		}
		if !c.Expires.IsZero() && (earliest.IsZero() || c.Expires.Before(earliest)) {
			earliest = c.Expires
		}
	}
	sort.Strings(v.CookieNames)
	v.ExpiresAt = formatTime(earliest)
	return v
}

// loginManager holds every login session for the life of the process.
type loginManager struct {
	mu       sync.Mutex
	browser  loginBrowser
	sessions map[string]*loginSession
	ttl      time.Duration
	closed   bool // set by closeAll: no window opens afterwards
}

func newLoginManager() *loginManager {
	return &loginManager{
		browser:  chromedpLoginBrowser{},
		sessions: make(map[string]*loginSession),
		ttl:      loginSessionTTL,
	}
}

// newLoginID is 128 random bits: a session id is the only thing standing between a crawl
// request and someone's logged-in cookies.
func newLoginID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// open registers a waiting session, then opens the browser without holding the lock.
func (m *loginManager) open(ctx context.Context, rawURL, host string) (loginView, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return loginView{}, errLoginShutdown
	}
	m.pruneLocked(time.Now())
	waiting := 0
	for _, s := range m.sessions {
		if s.state == loginWaiting {
			waiting++
		}
	}
	if waiting >= maxOpenLoginWindows {
		m.mu.Unlock()
		return loginView{}, errLoginLimit
	}
	s := &loginSession{id: newLoginID(), host: host, state: loginWaiting}
	m.sessions[s.id] = s
	browser, ttl := m.browser, m.ttl
	m.mu.Unlock()

	win, err := browser.Open(ctx, rawURL)

	m.mu.Lock()
	if err != nil {
		delete(m.sessions, s.id)
		m.mu.Unlock()
		return loginView{}, err
	}
	if m.sessions[s.id] != s {
		// Removed (DELETE, or closeAll at shutdown) while the browser was starting.
		m.mu.Unlock()
		_ = win.Close()
		return loginView{}, errLoginNotFound
	}
	s.window = win
	s.timer = time.AfterFunc(ttl, func() { m.expire(s.id) })
	v := s.view()
	m.mu.Unlock()
	go m.watch(s.id, win)
	return v, nil
}

// watch moves a waiting session to closed when the person closes its window, and cleans up
// the window's profile. Windows the manager closes itself are detached first, so they no-op.
func (m *loginManager) watch(id string, win loginWindow) {
	<-win.Done()
	m.mu.Lock()
	s, ok := m.sessions[id]
	userClosed := ok && s.window == win && s.state == loginWaiting
	if userClosed {
		s.state = loginClosed
		s.window = nil
		s.end()
	}
	m.mu.Unlock()
	if userClosed {
		_ = win.Close()
	}
}

func (m *loginManager) get(id string) (loginView, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return loginView{}, false
	}
	return s.view(), true
}

// capture reads the window's cookies, keeps the ones for the session's host, and closes the
// window. With no cookie for the host yet, the session stays waiting so the person can finish.
func (m *loginManager) capture(ctx context.Context, id string) (loginView, error) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return loginView{}, errLoginNotFound
	}
	if s.state != loginWaiting {
		st := s.state
		m.mu.Unlock()
		return loginView{}, loginStateErr(st)
	}
	win, host := s.window, s.host
	m.mu.Unlock()
	if win == nil {
		return loginView{}, &loginStateError{"the browser window is still opening; try again in a moment"}
	}

	jar, err := win.Cookies(ctx)
	if err != nil {
		select {
		case <-win.Done():
			// The window went away while we were reading it. Report what became of the
			// session (closed by the person, removed, expired, ...), not a guess.
			m.mu.Lock()
			cur, ok := m.sessions[id]
			var st loginState
			if ok {
				st = cur.state
				if st == loginWaiting {
					st = loginClosed // watch has not recorded the closure yet
				}
			}
			m.mu.Unlock()
			if !ok {
				return loginView{}, errLoginNotFound
			}
			return loginView{}, loginStateErr(st)
		default:
		}
		return loginView{}, err
	}
	kept := cookiesForHost(jar, host, time.Now())
	if len(kept) == 0 {
		return loginView{}, &loginStateError{fmt.Sprintf("the browser has no cookies for %s yet; finish logging in there, then try again", host)}
	}

	m.mu.Lock()
	if m.sessions[id] != s {
		m.mu.Unlock()
		return loginView{}, errLoginNotFound
	}
	if s.state != loginWaiting || s.window != win {
		st := s.state
		m.mu.Unlock()
		return loginView{}, loginStateErr(st)
	}
	s.state = loginCaptured
	s.cookies = kept
	s.window = nil
	v := s.view()
	m.mu.Unlock()
	_ = win.Close()
	return v, nil
}

// remove ends and forgets a session, closing its window.
func (m *loginManager) remove(id string) bool {
	m.mu.Lock()
	s, ok := m.sessions[id]
	var win loginWindow
	if ok {
		delete(m.sessions, id)
		s.end()
		s.cookies = nil
		win, s.window = s.window, nil
	}
	m.mu.Unlock()
	if win != nil {
		_ = win.Close()
	}
	return ok
}

// expire runs from the session's timer: it drops the cookies and closes any open window. The
// session stays listed as expired (until pruned) so the UI can say what happened.
func (m *loginManager) expire(id string) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok || (s.state != loginWaiting && s.state != loginCaptured) {
		m.mu.Unlock()
		return
	}
	win := s.window
	s.state = loginExpired
	s.window = nil
	s.cookies = nil
	s.endedAt = time.Now()
	m.mu.Unlock()
	if win != nil {
		_ = win.Close()
	}
}

// pruneLocked forgets ended sessions once they have been ended for a TTL.
func (m *loginManager) pruneLocked(now time.Time) {
	for id, s := range m.sessions {
		if (s.state == loginClosed || s.state == loginExpired) && now.Sub(s.endedAt) > m.ttl {
			delete(m.sessions, id)
		}
	}
}

// cookieHeader returns a captured session's cookies as a Cookie header value, and the host
// they belong to.
func (m *loginManager) cookieHeader(id string) (header, host string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return "", "", errLoginNotFound
	}
	if s.state != loginCaptured {
		return "", "", loginStateErr(s.state)
	}
	parts := make([]string, 0, len(s.cookies))
	for _, c := range cookiesForHost(s.cookies, s.host, time.Now()) {
		// Skip anything that isn't a valid name=value pair rather than sanitizing it:
		// net/http's sanitizer logs the bytes it drops, and these are secrets.
		if !httpguts.ValidHeaderFieldName(c.Name) || !httpguts.ValidHeaderFieldValue(c.Value) || strings.Contains(c.Value, ";") {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	if len(parts) == 0 {
		return "", "", &loginStateError{"the captured cookies have expired; log in again"}
	}
	return strings.Join(parts, "; "), s.host, nil
}

// closeAll closes every window, forgets every session and refuses new ones (process shutdown).
func (m *loginManager) closeAll() {
	m.mu.Lock()
	m.closed = true
	var wins []loginWindow
	for id, s := range m.sessions {
		if s.window != nil {
			wins = append(wins, s.window)
		}
		s.end()
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	for _, w := range wins {
		_ = w.Close()
	}
}

// cookiesForHost keeps the unexpired cookies a browser would send to host: a host-only cookie
// (CDP domain without a leading dot) only to that exact host, a domain cookie (".example.com")
// to that domain and its subdomains.
func cookiesForHost(jar []*http.Cookie, host string, now time.Time) []*http.Cookie {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	var out []*http.Cookie
	for _, c := range jar {
		if c == nil || c.Name == "" {
			continue
		}
		if !c.Expires.IsZero() && !c.Expires.After(now) {
			continue
		}
		if cookieDomainMatches(c.Domain, host) {
			out = append(out, c)
		}
	}
	return out
}

func cookieDomainMatches(domain, host string) bool {
	d := strings.ToLower(domain)
	if d == "" {
		return false
	}
	if rest, ok := strings.CutPrefix(d, "."); ok {
		return rest != "" && (host == rest || strings.HasSuffix(host, "."+rest))
	}
	return host == d
}

// chromedpLoginBrowser opens real windows through internal/render.
type chromedpLoginBrowser struct{}

func (chromedpLoginBrowser) Open(ctx context.Context, url string) (loginWindow, error) {
	w, err := render.OpenLoginWindow(ctx, url)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// loginProber is implemented by a loginBrowser that can tell, without opening a window,
// whether Open can work here. A browser that doesn't implement it is assumed available.
type loginProber interface {
	Available() error
}

func (chromedpLoginBrowser) Available() error { return render.LoginAvailable() }

// WithLoginBrowser replaces the Chromium launcher behind /api/login-sessions; tests pass a fake.
func WithLoginBrowser(b loginBrowser) Option {
	return func(s *Server) {
		if s.logins == nil {
			s.logins = newLoginManager()
		}
		s.logins.mu.Lock()
		s.logins.browser = b
		s.logins.mu.Unlock()
	}
}

// Close releases what the server holds outside its own memory: open login windows and their
// throwaway browser profiles. gocrawl serve calls it on shutdown, after the HTTP server has
// stopped; once Close has run, no login window can be opened.
func (s *Server) Close() {
	s.logins.closeAll()
}

const (
	maxLoginBody        = 16 << 10
	loginOpenTimeout    = 45 * time.Second
	loginCaptureTimeout = 30 * time.Second

	// StaleLoginProfileAge is how long a login window's throwaway browser profile must have
	// been untouched before SweepStaleLoginProfiles removes it.
	StaleLoginProfileAge = time.Hour
)

// SweepStaleLoginProfiles removes login-window profile directories (render.LoginProfilePrefix)
// in dir that were last modified more than maxAge ago: profiles a killed server (SIGKILL,
// crash) never got to delete. Only real directories are removed: a symlink with the prefix is
// left alone, and so is whatever it points at. Nothing else in dir is touched. Errors are
// ignored; this is housekeeping. gocrawl serve calls it once at startup, not New, so tests
// and embedders never sweep a temp directory they didn't ask about.
func SweepStaleLoginProfiles(dir string, maxAge time.Duration) {
	now := time.Now()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), render.LoginProfilePrefix) {
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) <= maxAge {
			continue
		}
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

type loginAvailability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (s *Server) loginRoutes() {
	s.mux.HandleFunc("GET /api/login-sessions/available", s.handleLoginAvailable)
	s.mux.HandleFunc("POST /api/login-sessions", s.handleStartLogin)
	s.mux.HandleFunc("GET /api/login-sessions/{id}", s.handleGetLogin)
	s.mux.HandleFunc("POST /api/login-sessions/{id}/capture", s.handleCaptureLogin)
	s.mux.HandleFunc("DELETE /api/login-sessions/{id}", s.handleDeleteLogin)
}

func (s *Server) handleLoginAvailable(w http.ResponseWriter, _ *http.Request) {
	s.logins.mu.Lock()
	browser := s.logins.browser
	s.logins.mu.Unlock()
	out := loginAvailability{Available: true}
	if p, ok := browser.(loginProber); ok {
		if err := p.Available(); err != nil {
			out = loginAvailability{Available: false, Reason: err.Error()}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleStartLogin(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var in struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBody)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	target, host, err := loginTarget(in.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), loginOpenTimeout)
	defer cancel()
	v, err := s.logins.open(ctx, target, host)
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, v)
	case errors.Is(err, errLoginLimit):
		writeError(w, http.StatusTooManyRequests, err)
	case errors.Is(err, errLoginShutdown):
		writeError(w, http.StatusServiceUnavailable, err)
	case errors.Is(err, render.ErrNoBrowser):
		writeError(w, http.StatusServiceUnavailable, errors.New("no Chromium-class browser found; install Google Chrome, Chromium or Microsoft Edge to log in with a browser"))
	case errors.Is(err, errLoginNotFound):
		writeError(w, http.StatusConflict, errors.New("the login was cancelled while the browser was starting"))
	default:
		// err comes from render.OpenLoginWindow, which already redacts URL userinfo; do not
		// add the request URL here.
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("could not open the login browser: %w", err))
	}
}

func (s *Server) handleGetLogin(w http.ResponseWriter, r *http.Request) {
	v, ok := s.logins.get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errLoginNotFound)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleCaptureLogin(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), loginCaptureTimeout)
	defer cancel()
	v, err := s.logins.capture(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, loginErrStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleDeleteLogin(w http.ResponseWriter, r *http.Request) {
	if !s.logins.remove(r.PathValue("id")) {
		writeError(w, http.StatusNotFound, errLoginNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func loginErrStatus(err error) int {
	var stateErr *loginStateError
	var reqErr *loginRequestError
	switch {
	case errors.Is(err, errLoginNotFound):
		return http.StatusNotFound
	case errors.As(err, &stateErr):
		return http.StatusConflict
	case errors.As(err, &reqErr):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// loginTarget normalizes the URL to open like a crawl seed: https:// by default, and any
// user:password stripped so it never reaches the browser's address bar or history.
func loginTarget(raw string) (target, host string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("url is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	raw, _, _ = crawler.SanitizeSeed(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		// Don't echo the input: it may carry credentials.
		return "", "", errors.New("url must be an http or https URL with a host")
	}
	return u.String(), asciiHost(u.Hostname()), nil
}

// asciiHost is the lower-case ASCII (punycode) form of a host name, the form browsers store
// cookie domains in, so "bücher.example" and "xn--bcher-kva.example" compare equal. A host the
// IDNA lookup rules reject (an underscore, say) is compared as typed, lower-cased.
func asciiHost(host string) string {
	if a, err := idna.Lookup.ToASCII(host); err == nil && a != "" {
		return strings.ToLower(a)
	}
	return strings.ToLower(host)
}

// loginRequestError is a crawl request that can't use its login session as asked (HTTP 400).
type loginRequestError struct{ msg string }

func (e *loginRequestError) Error() string { return e.msg }

// applyLoginSession turns a captured login into the crawl's Cookie header. From there the
// existing cookie path applies: sent only to the seed host (plus subdomains when asked), never
// over an https -> http downgrade, and refused by headless rendering. Errors never carry a
// cookie value or the seed URL (which may hold credentials).
func (s *Server) applyLoginSession(cfg *config.Config, seed, id, cookie string) error {
	if strings.TrimSpace(cookie) != "" {
		return &loginRequestError{"login_session and cookie can't be combined: the login session supplies the Cookie header"}
	}
	if cfg.Render == "headless" {
		return &loginRequestError{`login_session needs render "raw": headless Chromium can't keep the session cookie on the seed host`}
	}
	header, host, err := s.logins.cookieHeader(id)
	if err != nil {
		return err
	}
	// Hostname() is the host without port or userinfo; compare it in ASCII form, as the
	// session's host was recorded.
	u, err := url.Parse(seed)
	if err != nil || asciiHost(u.Hostname()) != host {
		return &loginRequestError{fmt.Sprintf("this login session is for %s, not for the crawl's seed host; log in again for that site", host)}
	}
	cfg.Crawl.Cookie = header
	return nil
}
