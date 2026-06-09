package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// HTTPFetcher fetches pages over HTTP(S). It follows redirects manually so the full
// redirect chain is recorded on the resulting Page.
type HTTPFetcher struct {
	client       *http.Client
	userAgent    string
	maxBody      int64
	maxRedirects int
}

// NewHTTPFetcher builds a fetcher from the given options.
func NewHTTPFetcher(opts Options) *HTTPFetcher {
	maxBody := opts.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = 5 << 20
	}
	maxRedirects := opts.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 10
	}
	return &HTTPFetcher{
		client: &http.Client{
			Timeout: opts.Timeout,
			// Never auto-follow; we follow manually to capture every hop.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		userAgent:    opts.UserAgent,
		maxBody:      maxBody,
		maxRedirects: maxRedirects,
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

	for hop := 0; hop <= f.maxRedirects; hop++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			page.Err = err.Error()
			page.Duration = time.Since(start)
			return page, err
		}
		req.Header.Set("User-Agent", f.userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

		resp, err := f.client.Do(req)
		if err != nil {
			page.Err = err.Error()
			page.Duration = time.Since(start)
			return page, err
		}

		if isRedirectStatus(resp.StatusCode) {
			loc := resp.Header.Get("Location")
			resp.Body.Close()
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
			current = next
			continue
		}

		// Final (non-redirect) response.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, f.maxBody))
		resp.Body.Close()

		page.StatusCode = resp.StatusCode
		page.FinalURL = current
		page.Header = resp.Header
		page.ContentType = resp.Header.Get("Content-Type")
		page.Body = body
		page.Duration = time.Since(start)

		if isHTMLContentType(page.ContentType) {
			if doc, derr := goquery.NewDocumentFromReader(strings.NewReader(string(body))); derr == nil {
				page.Doc = doc
			}
		}
		return page, nil
	}

	page.FinalURL = current
	page.Err = "too many redirects"
	page.Duration = time.Since(start)
	return page, nil
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
