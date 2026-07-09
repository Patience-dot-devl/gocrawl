package crawler

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func wantBasicAuthHeader(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func TestFetchSendsBasicAuthToRequestedHost(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, "ok")
	}))
	defer ts.Close()

	f := &HTTPFetcher{
		client:        &http.Client{},
		ua:            NewUAPool(Options{}),
		maxBody:       1 << 20,
		maxRedirects:  5,
		basicAuthUser: "alice",
		basicAuthPass: "s3cret",
	}
	if _, err := f.Fetch(context.Background(), ts.URL); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if want := wantBasicAuthHeader("alice", "s3cret"); gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
}

func TestFetchOmitsBasicAuthWhenUnset(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, "ok")
	}))
	defer ts.Close()

	f := &HTTPFetcher{
		client:       &http.Client{},
		ua:           NewUAPool(Options{}),
		maxBody:      1 << 20,
		maxRedirects: 5,
	}
	if _, err := f.Fetch(context.Background(), ts.URL); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty", gotAuth)
	}
}

// TestFetchDoesNotLeakBasicAuthAcrossHostRedirect guards against a real credential leak: a page
// on the authenticated host redirecting to a different host must not carry the Authorization
// header to that other host. It dials both "hosts" to local httptest servers via a custom
// Transport so no real DNS/network access is needed.
func TestFetchDoesNotLeakBasicAuthAcrossHostRedirect(t *testing.T) {
	var originAuth, otherAuth string
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originAuth = r.Header.Get("Authorization")
		http.Redirect(w, r, "http://other.invalid/asset", http.StatusFound)
	}))
	defer origin.Close()
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, "ok")
	}))
	defer other.Close()

	otherAddr := strings.TrimPrefix(other.URL, "http://")
	dialer := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.HasPrefix(addr, "other.invalid:") {
				addr = otherAddr
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}

	f := &HTTPFetcher{
		client: &http.Client{
			Transport:     transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		ua:            NewUAPool(Options{}),
		maxBody:       1 << 20,
		maxRedirects:  5,
		basicAuthUser: "alice",
		basicAuthPass: "s3cret",
	}
	page, err := f.Fetch(context.Background(), origin.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(page.Redirects) != 1 {
		t.Fatalf("got %d redirects, want 1 (fetch didn't reach the cross-host target)", len(page.Redirects))
	}
	if want := wantBasicAuthHeader("alice", "s3cret"); originAuth != want {
		t.Errorf("origin Authorization = %q, want %q", originAuth, want)
	}
	if otherAuth != "" {
		t.Errorf("other-host Authorization = %q, want empty (credentials leaked across redirect)", otherAuth)
	}
}
