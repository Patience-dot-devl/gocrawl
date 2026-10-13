package webserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpguts"

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
			return loginView{}, loginStateErr(loginClosed)
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

// closeAll closes every window and forgets every session (process shutdown).
func (m *loginManager) closeAll() {
	m.mu.Lock()
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

func (s *Server) loginRoutes() {}
