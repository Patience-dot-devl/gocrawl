package webserver

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"
)

// fakeLoginWindow stands in for a browser window. userCloses simulates the person closing
// it: Done fires without Close being called.
type fakeLoginWindow struct {
	mu      sync.Mutex
	cookies []*http.Cookie
	closed  bool
	done    chan struct{}
	once    sync.Once
}

func (w *fakeLoginWindow) Cookies(context.Context) ([]*http.Cookie, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, errors.New("window closed")
	}
	return w.cookies, nil
}

func (w *fakeLoginWindow) Done() <-chan struct{} { return w.done }

func (w *fakeLoginWindow) Close() error {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	w.once.Do(func() { close(w.done) })
	return nil
}

func (w *fakeLoginWindow) userCloses() { w.once.Do(func() { close(w.done) }) }

func (w *fakeLoginWindow) setCookies(c ...*http.Cookie) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cookies = c
}

func (w *fakeLoginWindow) isClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

// fakeLoginBrowser records every Open. next seeds each new window's cookies; err makes Open
// fail; gate, when set, holds Open until closed; available is what Available reports.
type fakeLoginBrowser struct {
	mu        sync.Mutex
	opened    []string
	windows   []*fakeLoginWindow
	next      []*http.Cookie
	err       error
	gate      chan struct{}
	available error
}

func (b *fakeLoginBrowser) Open(ctx context.Context, url string) (loginWindow, error) {
	b.mu.Lock()
	gate := b.gate
	b.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return nil, b.err
	}
	w := &fakeLoginWindow{cookies: b.next, done: make(chan struct{})}
	b.opened = append(b.opened, url)
	b.windows = append(b.windows, w)
	return w, nil
}

func (b *fakeLoginBrowser) Available() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.available
}

func (b *fakeLoginBrowser) window(t *testing.T, i int) *fakeLoginWindow {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if i >= len(b.windows) {
		t.Fatalf("window %d was never opened (%d opened)", i, len(b.windows))
	}
	return b.windows[i]
}

func newTestLoginManager(b loginBrowser) *loginManager {
	m := newLoginManager()
	m.browser = b
	return m
}

func waitForLogin(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// The capture keeps exactly the cookies a browser would send to the seed host.
func TestCookiesForHostFollowsBrowserDomainRules(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	jar := []*http.Cookie{
		{Name: "host_only", Value: "1", Domain: "shop.example.com"},
		{Name: "parent_domain", Value: "2", Domain: ".example.com"},
		{Name: "same_domain_dot", Value: "3", Domain: ".shop.example.com"},
		{Name: "sibling", Value: "4", Domain: "accounts.example.com"},
		{Name: "child_host_only", Value: "5", Domain: "eu.shop.example.com"},
		{Name: "other_site", Value: "6", Domain: ".evil.com"},
		{Name: "suffix_trick", Value: "7", Domain: "evilshop.example.com"},
		{Name: "partial_label", Value: "8", Domain: ".hop.example.com"},
		{Name: "expired", Value: "9", Domain: "shop.example.com", Expires: now.Add(-time.Minute)},
		{Name: "future", Value: "10", Domain: "shop.example.com", Expires: now.Add(time.Hour)},
		{Name: "no_domain", Value: "11"},
		nil,
	}
	var got []string
	for _, c := range cookiesForHost(jar, "Shop.Example.com", now) {
		got = append(got, c.Name)
	}
	want := []string{"host_only", "parent_domain", "same_domain_dot", "future"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("kept %v, want %v", got, want)
	}
}

func TestLoginManagerCaptureKeepsSeedCookiesAndClosesWindow(t *testing.T) {
	b := &fakeLoginBrowser{}
	m := newTestLoginManager(b)
	ctx := context.Background()
	v, err := m.open(ctx, "https://shop.example.com/account", "shop.example.com")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if v.State != "waiting" || v.Host != "shop.example.com" || v.CookieCount != 0 {
		t.Fatalf("open view = %+v", v)
	}
	win := b.window(t, 0)
	win.setCookies(
		&http.Cookie{Name: "sid", Value: "s3cret", Domain: "shop.example.com", Expires: time.Now().Add(2 * time.Hour)},
		&http.Cookie{Name: "pref", Value: "x", Domain: ".example.com", Expires: time.Now().Add(time.Hour)},
		&http.Cookie{Name: "idp", Value: "y", Domain: "login.idp.example"},
	)
	v, err = m.capture(ctx, v.ID)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if v.State != "captured" || v.CookieCount != 2 || !reflect.DeepEqual(v.CookieNames, []string{"pref", "sid"}) {
		t.Errorf("captured view = %+v", v)
	}
	if v.ExpiresAt == "" {
		t.Error("ExpiresAt empty, want the earliest cookie expiry")
	}
	if !win.isClosed() {
		t.Error("window still open after capture")
	}
	header, host, err := m.cookieHeader(v.ID)
	if err != nil {
		t.Fatalf("cookieHeader: %v", err)
	}
	if header != "sid=s3cret; pref=x" || host != "shop.example.com" {
		t.Errorf("cookieHeader = %q for %q", header, host)
	}
}

func TestLoginManagerCaptureWithoutSeedCookiesKeepsWaiting(t *testing.T) {
	b := &fakeLoginBrowser{}
	m := newTestLoginManager(b)
	ctx := context.Background()
	v, err := m.open(ctx, "https://shop.example.com/", "shop.example.com")
	if err != nil {
		t.Fatal(err)
	}
	b.window(t, 0).setCookies(&http.Cookie{Name: "idp", Value: "y", Domain: "login.idp.example"})
	_, err = m.capture(ctx, v.ID)
	var se *loginStateError
	if !errors.As(err, &se) {
		t.Fatalf("capture err = %v, want a loginStateError", err)
	}
	if got, _ := m.get(v.ID); got.State != "waiting" {
		t.Errorf("state = %s, want waiting", got.State)
	}
	if b.window(t, 0).isClosed() {
		t.Error("window closed although nothing was captured")
	}
}

func TestLoginManagerUserClosingWindowEndsSession(t *testing.T) {
	b := &fakeLoginBrowser{}
	m := newTestLoginManager(b)
	ctx := context.Background()
	v, err := m.open(ctx, "https://shop.example.com/", "shop.example.com")
	if err != nil {
		t.Fatal(err)
	}
	win := b.window(t, 0)
	win.userCloses()
	waitForLogin(t, "state closed", func() bool { got, _ := m.get(v.ID); return got.State == "closed" })
	waitForLogin(t, "profile cleanup (Close)", win.isClosed)
	var se *loginStateError
	if _, err := m.capture(ctx, v.ID); !errors.As(err, &se) {
		t.Errorf("capture after close err = %v, want a loginStateError", err)
	}
	if _, _, err := m.cookieHeader(v.ID); !errors.As(err, &se) {
		t.Errorf("cookieHeader after close err = %v, want a loginStateError", err)
	}
}

func TestLoginManagerExpiresSessions(t *testing.T) {
	b := &fakeLoginBrowser{next: []*http.Cookie{{Name: "sid", Value: "1", Domain: "shop.example.com"}}}
	m := newTestLoginManager(b)
	m.ttl = 200 * time.Millisecond
	ctx := context.Background()
	waiting, err := m.open(ctx, "https://shop.example.com/", "shop.example.com")
	if err != nil {
		t.Fatal(err)
	}
	captured, err := m.open(ctx, "https://shop.example.com/", "shop.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.capture(ctx, captured.ID); err != nil {
		t.Fatal(err)
	}
	waitForLogin(t, "both sessions expired", func() bool {
		a, _ := m.get(waiting.ID)
		c, _ := m.get(captured.ID)
		return a.State == "expired" && c.State == "expired"
	})
	if !b.window(t, 0).isClosed() {
		t.Error("expired waiting session left its window open")
	}
	if got, _ := m.get(captured.ID); got.CookieCount != 0 {
		t.Errorf("expired session still holds %d cookies", got.CookieCount)
	}
	var se *loginStateError
	if _, _, err := m.cookieHeader(captured.ID); !errors.As(err, &se) {
		t.Errorf("cookieHeader after expiry err = %v, want a loginStateError", err)
	}
}

func TestLoginManagerCapsOpenWindows(t *testing.T) {
	m := newTestLoginManager(&fakeLoginBrowser{})
	ctx := context.Background()
	for i := 0; i < maxOpenLoginWindows; i++ {
		if _, err := m.open(ctx, "https://shop.example.com/", "shop.example.com"); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}
	if _, err := m.open(ctx, "https://shop.example.com/", "shop.example.com"); !errors.Is(err, errLoginLimit) {
		t.Errorf("open past the cap err = %v, want errLoginLimit", err)
	}
}

// A session cancelled while its browser is still starting must not leave a window behind.
func TestLoginManagerRemoveWhileOpening(t *testing.T) {
	b := &fakeLoginBrowser{gate: make(chan struct{})}
	m := newTestLoginManager(b)
	type result struct {
		v   loginView
		err error
	}
	done := make(chan result, 1)
	go func() {
		v, err := m.open(context.Background(), "https://shop.example.com/", "shop.example.com")
		done <- result{v, err}
	}()
	var id string
	waitForLogin(t, "session registered", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		for k := range m.sessions {
			id = k
		}
		return id != ""
	})
	if !m.remove(id) {
		t.Fatal("remove returned false for a registered session")
	}
	close(b.gate)
	r := <-done
	if !errors.Is(r.err, errLoginNotFound) {
		t.Errorf("open err = %v, want errLoginNotFound", r.err)
	}
	waitForLogin(t, "late window closed", func() bool { return b.window(t, 0).isClosed() })
}

func TestLoginManagerCloseAll(t *testing.T) {
	b := &fakeLoginBrowser{}
	m := newTestLoginManager(b)
	ctx := context.Background()
	v, err := m.open(ctx, "https://shop.example.com/", "shop.example.com")
	if err != nil {
		t.Fatal(err)
	}
	m.closeAll()
	if !b.window(t, 0).isClosed() {
		t.Error("closeAll left a window open")
	}
	if _, ok := m.get(v.ID); ok {
		t.Error("closeAll kept the session")
	}
}
