package crawler

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
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

type stubRoundTripper func(*http.Request) (*http.Response, error)

func (f stubRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// TestFetchDropsBasicAuthOnSchemeDowngrade guards against a real credential leak: a same-host
// redirect from https to http (a misconfigured redirect, or an on-path attacker stripping TLS)
// must not carry the Authorization header onto the cleartext request.
func TestFetchDropsBasicAuthOnSchemeDowngrade(t *testing.T) {
	var secureAuth, plainAuth string
	transport := stubRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme == "https" {
			secureAuth = req.Header.Get("Authorization")
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://secure.invalid/asset"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}
		plainAuth = req.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})

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
	page, err := f.Fetch(context.Background(), "https://secure.invalid/")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(page.Redirects) != 1 {
		t.Fatalf("got %d redirects, want 1 (fetch didn't reach the downgraded target)", len(page.Redirects))
	}
	if want := wantBasicAuthHeader("alice", "s3cret"); secureAuth != want {
		t.Errorf("https Authorization = %q, want %q", secureAuth, want)
	}
	if plainAuth != "" {
		t.Errorf("http Authorization = %q, want empty (credentials leaked on scheme downgrade)", plainAuth)
	}
}

// TestFetchKeepsBasicAuthOnSchemeUpgrade guards against a real usability bug: a same-host
// redirect from http to https (an extremely common pattern — e.g. a plain http seed that gets
// force-redirected to https) must still carry the Authorization header onto the encrypted
// request, since that's a scheme upgrade rather than a downgrade.
func TestFetchKeepsBasicAuthOnSchemeUpgrade(t *testing.T) {
	var plainAuth, secureAuth string
	transport := stubRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme == "http" {
			plainAuth = req.Header.Get("Authorization")
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"https://plain.invalid/asset"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}
		secureAuth = req.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})

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
	page, err := f.Fetch(context.Background(), "http://plain.invalid/")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(page.Redirects) != 1 {
		t.Fatalf("got %d redirects, want 1 (fetch didn't reach the upgraded target)", len(page.Redirects))
	}
	if want := wantBasicAuthHeader("alice", "s3cret"); plainAuth != want {
		t.Errorf("http Authorization = %q, want %q", plainAuth, want)
	}
	if want := wantBasicAuthHeader("alice", "s3cret"); secureAuth != want {
		t.Errorf("https Authorization = %q, want %q (credentials should survive a same-host scheme upgrade)", secureAuth, want)
	}
}
