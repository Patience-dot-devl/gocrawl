// Package httpx implements HTTP-level checks: status codes, redirect chains and loops,
// slow responses, and mixed content.
package httpx

import (
	"context"
	"strings"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// Analyzer checks HTTP responses and redirects.
type Analyzer struct {
	// SlowThreshold is the response duration above which a page is flagged.
	SlowThreshold time.Duration
}

// New returns an HTTP analyzer with a 2s slow-response threshold.
func New() *Analyzer { return &Analyzer{SlowThreshold: 2 * time.Second} }

func (Analyzer) Name() string { return "redirects" }
func (Analyzer) Description() string {
	return "HTTP status codes, redirect chains/loops, slow responses, mixed content"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, a.analyzePage)
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	url := p.RequestedURL
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "redirects", URL: url, Severity: sev, Code: code, Message: msg, Data: data})
	}

	// found_on records the page this URL was first discovered on, so a broken/unreachable
	// URL points back to where it was linked rather than just naming the URL that failed.
	found := func(data map[string]any) map[string]any {
		if p.Referrer != "" {
			if data == nil {
				data = map[string]any{}
			}
			data["found_on"] = p.Referrer
		}
		return data
	}

	if p.Err != "" {
		add(analyze.Error, "http-fetch-error", "Failed to fetch page: "+p.Err, found(nil))
		return issues
	}

	switch {
	case p.StatusCode >= 500:
		add(analyze.Error, "http-server-error", "Server error response", found(map[string]any{"status": p.StatusCode}))
	case p.StatusCode >= 400:
		add(analyze.Error, "http-client-error", "Client error response", found(map[string]any{"status": p.StatusCode}))
	}

	// Redirect chain and loops.
	if n := len(p.Redirects); n > 0 {
		seen := map[string]bool{normalize(p.RequestedURL): true}
		loop := false
		for _, r := range p.Redirects {
			t := normalize(r.To)
			if seen[t] {
				loop = true
				break
			}
			seen[t] = true
		}
		switch {
		case loop:
			add(analyze.Error, "http-redirect-loop", "Redirect loop detected", map[string]any{"chain": chain(p)})
		case n > 1:
			add(analyze.Warning, "http-redirect-chain", "Multiple redirects before final URL", map[string]any{"hops": n, "chain": chain(p)})
		default:
			add(analyze.Info, "http-redirect", "Page redirects", map[string]any{"to": p.FinalURL, "status": p.Redirects[0].Status})
		}
	}

	// Slow response.
	if a.SlowThreshold > 0 && p.Duration > a.SlowThreshold {
		add(analyze.Warning, "http-slow-response", "Response slower than threshold", map[string]any{"duration_ms": p.Duration.Milliseconds()})
	}

	// Mixed content: HTTPS page referencing HTTP subresources.
	if strings.HasPrefix(p.FinalURL, "https://") && p.IsHTML() {
		if insecure := insecureSubresources(p.Doc); len(insecure) > 0 {
			add(analyze.Warning, "http-mixed-content", "HTTPS page loads insecure (http://) resources", map[string]any{"count": len(insecure), "examples": insecure})
		}
	}

	return issues
}

func chain(p *crawler.Page) []string {
	out := []string{p.RequestedURL}
	for _, r := range p.Redirects {
		out = append(out, r.To)
	}
	return out
}

func insecureSubresources(doc *goquery.Document) []string {
	var out []string
	doc.Find("img[src], script[src], link[href]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		for _, attr := range []string{"src", "href"} {
			if v, ok := s.Attr(attr); ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(v)), "http://") {
				out = append(out, v)
			}
		}
		return len(out) < 5
	})
	return out
}

// normalize prepares a URL for loop detection. It deliberately preserves the
// trailing slash: "/foo" and "/foo/" are distinct URLs, and a 301 from one to
// the other (e.g. WordPress canonical redirects) is a benign single hop, not a
// loop. A real loop is the same URL reappearing in the chain (A → B → A).
func normalize(u string) string { return strings.TrimSpace(u) }
