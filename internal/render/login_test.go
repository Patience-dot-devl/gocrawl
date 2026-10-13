package render

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
)

func TestBrowserLocationsCoverEveryDesktopOS(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		if len(browserLocations(goos)) == 0 {
			t.Errorf("no browser locations for %s", goos)
		}
	}
	if !slices.Contains(browserLocations("darwin"), "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome") {
		t.Error("darwin locations miss Google Chrome")
	}
	if !slices.Contains(browserLocations("windows"), "msedge") {
		t.Error("windows locations miss Edge")
	}
	if slices.Contains(browserLocations("linux"), "headless_shell") {
		t.Error("linux locations include headless_shell, which can't show a login window")
	}
}

func TestLoginAvailableReportsMissingBrowser(t *testing.T) {
	if _, err := FindBrowser(); err != nil {
		if !errors.Is(err, ErrNoBrowser) {
			t.Fatalf("FindBrowser err = %v, want ErrNoBrowser", err)
		}
		if err := LoginAvailable(); !errors.Is(err, ErrNoBrowser) {
			t.Errorf("LoginAvailable = %v, want ErrNoBrowser", err)
		}
	}
}

// httpCookies keeps the value (the point of a login capture, unlike convertCookies, which
// drops it) and CDP's domain spelling: a leading dot marks a domain cookie.
func TestHTTPCookiesKeepsValuesAndDomainForm(t *testing.T) {
	expires := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	got := httpCookies([]*network.Cookie{
		{Name: "sid", Value: "abc", Domain: "shop.example.com", Path: "/", HTTPOnly: true, Secure: true, Session: true, Expires: -1},
		nil,
		{Name: "pref", Value: "1", Domain: ".example.com", Path: "/", Expires: float64(expires.Unix())},
	})
	if len(got) != 2 {
		t.Fatalf("got %d cookies, want 2", len(got))
	}
	sid, pref := got[0], got[1]
	if sid.Name != "sid" || sid.Value != "abc" || sid.Domain != "shop.example.com" || !sid.HttpOnly || !sid.Secure || !sid.Expires.IsZero() {
		t.Errorf("sid = %+v", sid)
	}
	if pref.Domain != ".example.com" || !pref.Expires.Equal(expires) {
		t.Errorf("pref = %+v, want domain .example.com expiring %v", pref, expires)
	}
}

// Real browser, skipped without one like the headless render tests. It runs the window
// headless (visible=false) so `go test` never pops a window up; visibility only changes the
// allocator flags.
func TestLoginWindowReadsHTTPOnlyCookiesAndRemovesProfile(t *testing.T) {
	if _, err := FindBrowser(); err != nil {
		t.Skip("no Chromium-class browser available")
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s3cret", Path: "/", HttpOnly: true})
		fmt.Fprint(w, `<html><body>signed in</body></html>`)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	w, err := openLoginWindow(ctx, ts.URL, false)
	if err != nil {
		t.Skipf("could not launch the browser: %v", err)
	}
	dir := w.profileDir
	if _, err := os.Stat(dir); err != nil {
		_ = w.Close()
		t.Fatalf("profile dir %s missing while the window is open: %v", dir, err)
	}

	cookies, err := w.Cookies(ctx)
	if err != nil {
		_ = w.Close()
		t.Fatalf("Cookies: %v", err)
	}
	var sid *http.Cookie
	for _, c := range cookies {
		if c.Name == "sid" {
			sid = c
		}
	}
	if sid == nil || sid.Value != "s3cret" || !sid.HttpOnly || sid.Domain != "127.0.0.1" {
		t.Errorf("sid cookie = %+v, want the HttpOnly value s3cret for 127.0.0.1", sid)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-w.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("Done not closed after Close")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("profile dir %s still exists after Close (stat err %v)", dir, err)
	}
	if _, err := w.Cookies(ctx); !errors.Is(err, ErrLoginWindowClosed) {
		t.Errorf("Cookies after Close err = %v, want ErrLoginWindowClosed", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}
