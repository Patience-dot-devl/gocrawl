package crawler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
)

// HTTPFetcher fetches pages over HTTP(S). It follows redirects manually so the full
// redirect chain is recorded on the resulting Page.
type HTTPFetcher struct {
	client        *http.Client
	ua            *UAPool
	maxBody       int64
	maxRedirects  int
	basicAuthUser string
	basicAuthPass string

	// allowRedirect, when set, gates each redirect hop against crawl scope, exclude rules,
	// and robots.txt — the same check applied to a URL before it's ever enqueued. Without
	// it, a redirect can hop to any host or path (a relevant SSRF surface when gocrawl runs
	// as an MCP server) and the target is fetched and analyzed as an ordinary page. Set by
	// Engine.New for the raw fetcher it drives; left nil (unrestricted) for one-off fetchers
	// such as the robots.txt fetcher, which has no crawl scope to check against.
	allowRedirect func(ctx context.Context, u *url.URL) bool
}

// NewHTTPFetcher builds a fetcher from the given options. When opts.Proxies is non-empty the
// client routes each request through a proxy chosen by opts.ProxyRotation; otherwise Go's
// default proxy behavior (environment variables) applies. The User-Agent header is chosen per
// request from opts.UserAgents / opts.UserAgent via opts.UserAgentRotation.
func NewHTTPFetcher(opts Options) *HTTPFetcher {
	maxBody := opts.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = 5 << 20
	}
	maxRedirects := opts.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 10
	}
	client := &http.Client{
		Timeout: opts.Timeout,
		// Never auto-follow; we follow manually to capture every hop.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if pp := newProxyPool(opts); pp != nil {
		// Clone the default transport so connection-pool tuning is preserved, then swap in the
		// rotating proxy selector. Connections are pooled per (proxy, target), so rotation works
		// cleanly alongside keep-alive.
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = pp.proxyFunc()
		client.Transport = transport
	}
	return &HTTPFetcher{
		client:        client,
		ua:            NewUAPool(opts),
		maxBody:       maxBody,
		maxRedirects:  maxRedirects,
		basicAuthUser: opts.BasicAuthUser,
		basicAuthPass: opts.BasicAuthPass,
	}
}

func isRedirectStatus(code int) bool {
	switch code {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func isHTMLContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

// Fetch retrieves rawURL, following redirects manually up to maxRedirects.
func (f *HTTPFetcher) Fetch(ctx context.Context, rawURL string) (*Page, error) {
	page := &Page{RequestedURL: rawURL, FetchedAt: time.Now()}
	start := time.Now()
	current := rawURL
	origHost, origScheme := "", ""
	if u, err := url.Parse(rawURL); err == nil {
		origHost = u.Hostname()
		origScheme = u.Scheme
	}

	for hop := 0; hop <= f.maxRedirects; hop++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			page.Err = err.Error()
			page.Duration = time.Since(start)
			return page, err
		}
		if ua := f.ua.Next(req.URL.Hostname()); ua != "" {
			req.Header.Set("User-Agent", ua)
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		// Only sent to the host we were asked to fetch, and never downgraded to plain HTTP
		// once escalated to HTTPS, so credentials for the crawled site can't leak to a
		// redirect target on another domain or over the wire in cleartext. An http seed that
		// redirects to https on the same host (an extremely common pattern) is allowed to
		// carry auth forward, since that's a scheme upgrade, not a downgrade.
		schemeOK := req.URL.Scheme == origScheme || (origScheme == "http" && req.URL.Scheme == "https")
		if f.basicAuthUser != "" && req.URL.Hostname() == origHost && schemeOK {
			req.SetBasicAuth(f.basicAuthUser, f.basicAuthPass)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			page.Err = err.Error()
			page.Duration = time.Since(start)
			return page, err
		}

		if isRedirectStatus(resp.StatusCode) {
			loc := resp.Header.Get("Location")
			_ = resp.Body.Close()
			if loc == "" {
				page.StatusCode = resp.StatusCode
				page.FinalURL = current
				page.Err = "redirect without Location header"
				page.Duration = time.Since(start)
				return page, nil
			}
			next, ok := resolveLocation(current, loc)
			if !ok {
				page.Err = fmt.Sprintf("invalid redirect target %q", loc)
				page.Duration = time.Since(start)
				return page, nil
			}
			page.Redirects = append(page.Redirects, Redirect{From: current, To: next, Status: resp.StatusCode})
			if f.allowRedirect != nil {
				nu, perr := url.Parse(next)
				if perr != nil || !f.allowRedirect(ctx, nu) {
					page.FinalURL = current
					page.Err = fmt.Sprintf("redirect to %q blocked by crawl scope, exclude rules, or robots.txt", next)
					page.Duration = time.Since(start)
					return page, nil
				}
			}
			current = next
			continue
		}

		// Final (non-redirect) response. Read one byte past the cap so a response that's
		// exactly maxBody long isn't mistaken for a truncated one: only exceeding the cap
		// means real content was cut off. A read error partway through the body is the same
		// class of problem — Body is incomplete either way — so it's flagged the same way
		// rather than silently discarded.
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, f.maxBody+1))
		_ = resp.Body.Close()

		truncated := readErr != nil
		if int64(len(body)) > f.maxBody {
			body = body[:f.maxBody]
			truncated = true
		}

		page.StatusCode = resp.StatusCode
		page.FinalURL = current
		page.Header = resp.Header
		page.ContentType = resp.Header.Get("Content-Type")
		page.Truncated = truncated
		page.Duration = time.Since(start)

		if isHTMLContentType(page.ContentType) {
			// Decode to UTF-8 before anything else sees the body: without this, a page
			// served in e.g. Windows-1252 or Shift_JIS is parsed as if it were UTF-8,
			// corrupting every multi-byte/high-byte character into mojibake for every
			// analyzer that reads Body or Doc.Text().
			body = decodeToUTF8(body, page.ContentType)
			if doc, derr := goquery.NewDocumentFromReader(strings.NewReader(string(body))); derr == nil {
				page.Doc = doc
			}
		}
		page.Body = body
		return page, nil
	}

	page.FinalURL = current
	page.Err = "too many redirects"
	page.Duration = time.Since(start)
	return page, nil
}

// decodeToUTF8 detects body's encoding — from the Content-Type header's charset param, a BOM,
// or a sniffed <meta charset>/<meta http-equiv> declaration, falling back to a UTF-8-validity
// check and then windows-1252 per the HTML5 spec — and transcodes it to UTF-8. Already-UTF-8
// content (declared or merely valid) passes through unchanged. If transcoding fails for any
// reason, the original bytes are returned rather than dropping the page.
func decodeToUTF8(body []byte, contentType string) []byte {
	r, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return body
	}
	decoded, err := io.ReadAll(r)
	if err != nil || len(decoded) == 0 {
		return body
	}
	return decoded
}

func resolveLocation(base, loc string) (string, bool) {
	b, err := url.Parse(base)
	if err != nil {
		return "", false
	}
	ref, err := url.Parse(strings.TrimSpace(loc))
	if err != nil {
		return "", false
	}
	abs := b.ResolveReference(ref)
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return "", false
	}
	return abs.String(), true
}
