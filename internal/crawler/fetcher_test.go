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
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
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

// TestFetchOmitsBasicAuthWhenAuthHostAllowedRejects guards against a real credential leak: a
// crawl run with FollowExternal set stops enforcing the seed-host check in inScope, so without
// its own independent guard the fetcher would send the seed's Basic Auth to every external host
// it's asked to fetch — not just on a redirect, but on the very first request. authHostAllowed
// is that independent guard; Engine.New wires it regardless of FollowExternal (see
// TestEngineWiresAuthHostAllowedOnHTTPFetcher).
func TestFetchOmitsBasicAuthWhenAuthHostAllowedRejects(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, "ok")
	}))
	defer ts.Close()

	f := &HTTPFetcher{
		client:          &http.Client{},
		ua:              NewUAPool(Options{}),
		maxBody:         1 << 20,
		maxRedirects:    5,
		basicAuthUser:   "alice",
		basicAuthPass:   "s3cret",
		authHostAllowed: func(host string) bool { return false }, // simulates an external host under FollowExternal
	}
	if _, err := f.Fetch(context.Background(), ts.URL); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty (credentials leaked to a host authHostAllowed rejected)", gotAuth)
	}
}

// TestFetchSendsBasicAuthToIPv6SeedHost guards against a real regression: authHostAllowed must
// be fed a bracket-preserving host (req.URL.Host, like every other sameSite caller uses — e.g.
// e.seedHost, inScope's u.Host), not req.URL.Hostname(), which strips IPv6 brackets. Passing
// Hostname() here would make sameSite compare "[::1]" against "::1", never match, and silently
// drop Basic Auth on every request to an IPv6-literal seed.
func TestFetchSendsBasicAuthToIPv6SeedHost(t *testing.T) {
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback not available: %v", err)
	}
	var gotAuth string
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, "ok")
	}))
	ts.Listener = ln
	ts.Start()
	defer ts.Close()

	seedURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse %q: %v", ts.URL, err)
	}

	f := &HTTPFetcher{
		client:          &http.Client{},
		ua:              NewUAPool(Options{}),
		maxBody:         1 << 20,
		maxRedirects:    5,
		basicAuthUser:   "alice",
		basicAuthPass:   "s3cret",
		authHostAllowed: func(host string) bool { return sameSite(seedURL.Host, host, false) },
	}
	if _, err := f.Fetch(context.Background(), ts.URL); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if want := wantBasicAuthHeader("alice", "s3cret"); gotAuth != want {
		t.Errorf("Authorization = %q, want %q (dropped for an IPv6-literal seed)", gotAuth, want)
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

// windows1252 encodes s (assumed to contain only characters representable in windows-1252)
// into its windows-1252 byte form, for building non-UTF-8 test fixtures.
func windows1252(t *testing.T, s string) []byte {
	t.Helper()
	b, err := charmap.Windows1252.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatalf("encoding %q as windows-1252: %v", s, err)
	}
	return b
}

// TestFetchDecodesWindows1252ToUTF8 guards against a real correctness bug: without charset
// detection, a non-UTF-8 page (declared via its Content-Type header, here) was parsed as if
// it were UTF-8, corrupting every high-byte character into mojibake for every analyzer that
// reads Body or Doc.Text().
func TestFetchDecodesWindows1252ToUTF8(t *testing.T) {
	const title = "Café Müller"
	html := "<html><head><title>" + title + "</title></head><body>" + title + "</body></html>"
	body := windows1252(t, html)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=windows-1252")
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	f := &HTTPFetcher{
		client:       &http.Client{},
		ua:           NewUAPool(Options{}),
		maxBody:      1 << 20,
		maxRedirects: 5,
	}
	page, err := f.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !utf8.Valid(page.Body) {
		t.Error("page.Body is not valid UTF-8")
	}
	if !strings.Contains(string(page.Body), title) {
		t.Errorf("page.Body doesn't contain the decoded title %q: %q", title, page.Body)
	}
	if page.Doc == nil {
		t.Fatal("expected a parsed Doc")
	}
	if got := strings.TrimSpace(page.Doc.Find("title").Text()); got != title {
		t.Errorf("parsed title = %q, want %q (mojibake if this fails)", got, title)
	}
}

// TestFetchLeavesDeclaredUTF8Unchanged confirms the fix doesn't corrupt the common case: HTML
// already served as UTF-8 (declared or merely valid) must pass through byte-for-byte.
func TestFetchLeavesDeclaredUTF8Unchanged(t *testing.T) {
	const title = "Café Müller" // valid UTF-8, no charset declared in the Content-Type header
	html := "<html><head><title>" + title + "</title></head><body>" + title + "</body></html>"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	}))
	defer ts.Close()

	f := &HTTPFetcher{
		client:       &http.Client{},
		ua:           NewUAPool(Options{}),
		maxBody:      1 << 20,
		maxRedirects: 5,
	}
	page, err := f.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(page.Body) != html {
		t.Errorf("page.Body = %q, want unchanged %q", page.Body, html)
	}
}
