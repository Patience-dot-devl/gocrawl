package render

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// LoginProfilePrefix is the temp-directory name prefix of a login window's throwaway browser
// profile; webserver.SweepStaleLoginProfiles recognizes leftovers by it.
const LoginProfilePrefix = "gocrawl-login-"

var (
	// ErrNoBrowser means no Chromium-class browser is installed where gocrawl looks.
	ErrNoBrowser = errors.New("no Chromium-class browser found (install Google Chrome, Chromium or Microsoft Edge)")
	// ErrNoDisplay means a visible window can't open: no X11 or Wayland display.
	ErrNoDisplay = errors.New("no display for a browser window (DISPLAY and WAYLAND_DISPLAY are both unset)")
	// ErrLoginWindowClosed is returned by Cookies once the window is gone.
	ErrLoginWindowClosed = errors.New("the browser window was closed")
)

// browserLocations lists, per OS, the browser names and paths FindBrowser tries in order:
// chromedp's own list (what headless render launches through), minus headless_shell, which
// has no window, plus Microsoft Edge where chromedp only looks for it on Windows.
func browserLocations(goos string) []string {
	switch goos {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "windows":
		return []string{
			"chrome",
			"chrome.exe",
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			filepath.Join(os.Getenv("USERPROFILE"), `AppData\Local\Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("USERPROFILE"), `AppData\Local\Chromium\Application\chrome.exe`),
			"msedge",
			"msedge.exe",
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		}
	default:
		return []string{
			"chromium",
			"chromium-browser",
			"google-chrome",
			"google-chrome-stable",
			"google-chrome-beta",
			"google-chrome-unstable",
			"/usr/bin/google-chrome",
			"/usr/local/bin/chrome",
			"/snap/bin/chromium",
			"chrome",
			"microsoft-edge",
			"microsoft-edge-stable",
		}
	}
}

// FindBrowser returns the path of the first Chromium-class browser it finds, or ErrNoBrowser.
func FindBrowser() (string, error) {
	for _, p := range browserLocations(runtime.GOOS) {
		if found, err := exec.LookPath(p); err == nil {
			return found, nil
		}
	}
	return "", ErrNoBrowser
}

// LoginAvailable reports why OpenLoginWindow can't work on this machine, or nil when it can.
func LoginAvailable() error {
	if _, err := FindBrowser(); err != nil {
		return err
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" &&
		os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return ErrNoDisplay
	}
	return nil
}

// LoginWindow is a browser window opened for a person to log in to a site by hand, in a
// throwaway profile directory that Close deletes. gocrawl only reads its cookie jar.
type LoginWindow struct {
	profileDir  string
	allocCancel context.CancelFunc
	tabCtx      context.Context
	tabCancel   context.CancelFunc

	done      chan struct{}
	doneOnce  sync.Once
	closeOnce sync.Once
	closeErr  error
}

// OpenLoginWindow starts a visible browser on rawURL. ctx bounds only the launch and the first
// navigation: the window stays open until Close, the person closes it, or the process exits.
func OpenLoginWindow(ctx context.Context, rawURL string) (*LoginWindow, error) {
	if err := LoginAvailable(); err != nil {
		return nil, err
	}
	return openLoginWindow(ctx, rawURL, true)
}

func openLoginWindow(ctx context.Context, rawURL string, visible bool) (*LoginWindow, error) {
	path, err := FindBrowser()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", LoginProfilePrefix)
	if err != nil {
		return nil, fmt.Errorf("creating a temporary browser profile: %w", err)
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(path), chromedp.UserDataDir(dir))
	if visible {
		opts = append(opts, chromedp.VisibleWindow)
	}
	// The browser must outlive the HTTP request that opened it, so its contexts hang off
	// Background, not ctx.
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	tabCtx, tabCancel := chromedp.NewContext(allocCtx, chromedp.WithErrorf(quietErrorf))
	w := &LoginWindow{
		profileDir:  dir,
		allocCancel: allocCancel,
		tabCtx:      tabCtx,
		tabCancel:   tabCancel,
		done:        make(chan struct{}),
	}

	// The first action starts the browser and ties it to the context it runs on, so it runs
	// on tabCtx itself; ctx only bounds how long we wait for it.
	started := make(chan error, 1)
	go func() { started <- chromedp.Do(tabCtx) }()
	select {
	case err := <-started:
		if err != nil {
			_ = w.Close()
			if errors.Is(err, chromedp.ErrNoDisplay) {
				return nil, ErrNoDisplay
			}
			return nil, fmt.Errorf("launching the browser: %w", err)
		}
	case <-ctx.Done():
		// Stop chromedp's launch and wait for it to return before Close deletes the profile,
		// or the still-starting browser could recreate the directory after the delete.
		w.tabCancel()
		<-started
		_ = w.Close()
		return nil, fmt.Errorf("launching the browser: %w", ctx.Err())
	}

	c := chromedp.FromContext(tabCtx)
	go w.watch(c.Browser.LostConnection, c.Target.TargetID)

	navCtx, navCancel := context.WithCancel(tabCtx)
	defer navCancel()
	stop := context.AfterFunc(ctx, navCancel)
	defer stop()
	if err := chromedp.Do(navCtx, chromedp.Navigate(rawURL)); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("opening %s: %w", redactURL(rawURL), err)
	}
	return w, nil
}

// redactURL returns rawURL with any password in its userinfo masked, so a URL pasted as
// https://user:pass@host never carries the password into an error message. An unparsable URL
// is dropped entirely rather than echoed.
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "the given URL"
	}
	return u.Redacted()
}

// watch closes done when the person quits the browser (lost connection) or closes the tab
// (Target.targetDestroyed for our target; on macOS the browser keeps running windowless).
func (w *LoginWindow) watch(lost <-chan struct{}, id target.ID) {
	gone := make(chan struct{})
	go func() {
		for ev, err := range chromedp.BrowserEvents(w.tabCtx, target.TargetDestroyed) {
			if err != nil {
				return
			}
			if ev.TargetID == id {
				close(gone)
				return
			}
		}
	}()
	select {
	case <-lost:
	case <-gone:
	case <-w.tabCtx.Done():
	}
	w.doneOnce.Do(func() { close(w.done) })
}

// Done is closed once the window is gone, by Close or by the person.
func (w *LoginWindow) Done() <-chan struct{} { return w.done }

// Cookies reads the browser's whole cookie jar over CDP, HttpOnly cookies and values
// included. Filtering to the site is the caller's job.
func (w *LoginWindow) Cookies(ctx context.Context) ([]*http.Cookie, error) {
	select {
	case <-w.done:
		return nil, ErrLoginWindowClosed
	default:
	}
	callCtx, cancel := context.WithTimeout(w.tabCtx, 15*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	var jar []*network.Cookie
	err := chromedp.Do(callCtx, chromedp.Func(func(c context.Context, t *chromedp.Target) error {
		res, err := cdp.Call(c, t, storage.GetCookies, storage.GetCookiesParams{})
		if err != nil {
			return err
		}
		jar = res.Cookies
		return nil
	}))
	if err != nil {
		select {
		case <-w.done:
			return nil, ErrLoginWindowClosed
		default:
		}
		return nil, fmt.Errorf("reading the browser's cookies: %w", err)
	}
	return httpCookies(jar), nil
}

// Close stops the browser and deletes the throwaway profile. Safe to call more than once.
func (w *LoginWindow) Close() error {
	w.closeOnce.Do(func() {
		w.tabCancel()
		w.allocCancel()
		w.doneOnce.Do(func() { close(w.done) })
		w.closeErr = removeProfileDir(w.profileDir)
	})
	return w.closeErr
}

// removeProfileDir retries for a few seconds: Chromium's helper processes can hold files in
// the profile briefly after the main process exits.
func removeProfileDir(dir string) error {
	var err error
	for range 50 {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("removing the temporary browser profile %s: %w", dir, err)
}

// httpCookies converts CDP cookies to net/http ones, keeping the value and CDP's domain
// spelling (leading dot = domain cookie, none = host-only). CDP reports a session cookie's
// expiry as -1; it maps to the zero time.
func httpCookies(jar []*network.Cookie) []*http.Cookie {
	out := make([]*http.Cookie, 0, len(jar))
	for _, c := range jar {
		if c == nil {
			continue
		}
		hc := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		}
		if !c.Session && c.Expires > 0 {
			hc.Expires = time.Unix(int64(c.Expires), 0).UTC()
		}
		out = append(out, hc)
	}
	return out
}
