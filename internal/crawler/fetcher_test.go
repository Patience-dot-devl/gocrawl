package crawler

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// TestFetchBlocksRedirectRejectedByAllowRedirect guards against a real SSRF/scope-escape
// surface: without a scope check, a redirect can hop the fetcher to any host or path (e.g. an
// internal/metadata endpoint reachable from an MCP server) and the target would be fetched and
// analyzed as an ordinary page. When allowRedirect rejects a hop, the fetcher must stop
// following it rather than fetch the target.
func TestFetchBlocksRedirectRejectedByAllowRedirect(t *testing.T) {
	var targetHit bool
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/blocked", http.StatusFound)
	})
	mux.HandleFunc("/blocked", func(w http.ResponseWriter, _ *http.Request) {
		targetHit = true
		fmt.Fprint(w, "should never be reached")
	})
	origin := httptest.NewServer(mux)
	defer origin.Close()

	f := &HTTPFetcher{
		client: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		ua:           NewUAPool(Options{}),
		maxBody:      1 << 20,
		maxRedirects: 5,
		allowRedirect: func(_ context.Context, u *url.URL) bool {
			return !strings.Contains(u.Path, "/blocked")
		},
	}
	page, err := f.Fetch(context.Background(), origin.URL+"/")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if targetHit {
		t.Error("blocked redirect target was fetched")
	}
	if page.Err == "" {
		t.Error("expected page.Err to explain the blocked redirect")
	}
	if len(page.Redirects) != 1 {
		t.Errorf("expected the blocked hop to still be recorded in Redirects, got %d", len(page.Redirects))
	}
}

// TestFetchAllowsRedirectWhenAllowRedirectNil confirms the default (no scope check configured,
// e.g. the one-off fetcher used for robots.txt) behaves exactly as before: redirects are
// followed unconditionally.
func TestFetchAllowsRedirectWhenAllowRedirectNil(t *testing.T) {
	var targetHit bool
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/target", http.StatusFound)
	})
	mux.HandleFunc("/target", func(w http.ResponseWriter, _ *http.Request) {
		targetHit = true
		fmt.Fprint(w, "ok")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	f := &HTTPFetcher{
		client: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		ua:           NewUAPool(Options{}),
		maxBody:      1 << 20,
		maxRedirects: 5,
	}
	page, err := f.Fetch(context.Background(), ts.URL+"/")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !targetHit {
		t.Error("expected the redirect target to be fetched when no allowRedirect is set")
	}
	if len(page.Redirects) != 1 {
		t.Errorf("got %d redirects, want 1", len(page.Redirects))
	}
	if page.Err != "" {
		t.Errorf("unexpected page.Err: %q", page.Err)
	}
}

// TestFetchFlagsTruncatedBody guards against a real correctness bug: a body cut off at the
// fetcher's size cap was silently treated as a complete page, so downstream analyzers (e.g.
// "missing title") could misreport content that was simply never read.
func TestFetchFlagsTruncatedBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, strings.Repeat("x", 100))
	}))
	defer ts.Close()

	f := &HTTPFetcher{
		client:       &http.Client{},
		ua:           NewUAPool(Options{}),
		maxBody:      50,
		maxRedirects: 5,
	}
	page, err := f.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !page.Truncated {
		t.Error("expected Truncated=true for a body over the cap")
	}
	if len(page.Body) != 50 {
		t.Errorf("got %d body bytes, want exactly the 50-byte cap", len(page.Body))
	}
}

// TestFetchDoesNotFlagBodyExactlyAtCap ensures a response that happens to be exactly maxBody
// long — not actually truncated — isn't mistaken for one.
func TestFetchDoesNotFlagBodyExactlyAtCap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, strings.Repeat("x", 50))
	}))
	defer ts.Close()

	f := &HTTPFetcher{
		client:       &http.Client{},
		ua:           NewUAPool(Options{}),
		maxBody:      50,
		maxRedirects: 5,
	}
	page, err := f.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if page.Truncated {
		t.Error("a response exactly at the cap should not be flagged Truncated")
	}
	if len(page.Body) != 50 {
		t.Errorf("got %d body bytes, want 50", len(page.Body))
	}
}
