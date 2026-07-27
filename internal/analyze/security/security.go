// Package security implements security checks. It runs in two passes.
//
// The default pass is per-page and passive: the three baseline response headers (HSTS, CSP,
// X-Content-Type-Options) and forms whose action downgrades to http://.
//
// The opt-in pass is the security audit — transport-layer and cookie hygiene: the TLS
// protocol version and cipher suite, the certificate chain the server presented (expiry,
// signature and key strength, completeness), Set-Cookie attributes, and the deeper
// response-header policy checks (HSTS quality, framing protection, referrer policy, software
// version disclosure). It is enabled with WithAudit, wired to the `security_audit` config
// flag, so the default analyzer stays minimal.
//
// Everything the audit inspects is server configuration, identical across every page a host
// serves, so audit findings are aggregated and emitted once per host — reported against the
// first page crawled on that host — rather than repeating on all of them. The baseline
// per-page checks keep their existing per-page behaviour.
//
// The audit reads only what the crawl already fetched: it opens no extra connections and
// sends no probes. That bounds what it can see — a certificate so broken that Go's TLS stack
// refuses the handshake produces a fetch error, not an audit finding.
package security

import (
	"context"
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// Analyzer performs security header and insecure-form checks, plus the opt-in TLS,
// certificate, and cookie audit.
type Analyzer struct {
	audit bool
}

// Option configures the analyzer.
type Option func(*Analyzer)

// WithAudit enables the opt-in security audit: TLS/certificate inspection, Set-Cookie
// attribute checks, and deeper response-header hygiene. Off by default, since these findings
// speak to server and platform configuration rather than to the on-page SEO work that drives
// a default crawl.
func WithAudit(on bool) Option { return func(a *Analyzer) { a.audit = on } }

// New returns a new security analyzer.
func New(opts ...Option) *Analyzer {
	a := &Analyzer{}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (Analyzer) Name() string { return "security" }
func (Analyzer) Description() string {
	return "Security headers (HSTS, CSP, X-Content-Type-Options), insecure forms, and — with --security-audit — TLS, certificate, and cookie checks"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	issues := analyze.EachPage(result, a.analyzePage)
	if a.audit {
		issues = append(issues, a.auditSite(result)...)
	}
	return issues
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	final := p.FinalURL
	doc := p.Doc
	https := false
	if u, err := url.Parse(final); err == nil {
		https = u.Scheme == "https"
	}
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "security", URL: final, Severity: sev, Code: code, Message: msg, Data: data})
	}

	// Header checks are nil-safe: p.Header may be nil in tests.
	if p.Header != nil {
		if https && p.Header.Get("Strict-Transport-Security") == "" {
			add(analyze.Warning, "security-missing-hsts", "HTTPS response has no Strict-Transport-Security header", nil)
		}
		if p.Header.Get("Content-Security-Policy") == "" {
			add(analyze.Info, "security-missing-csp", "Response has no Content-Security-Policy header", nil)
		}
		if !strings.Contains(strings.ToLower(p.Header.Get("X-Content-Type-Options")), "nosniff") {
			add(analyze.Info, "security-missing-x-content-type-options", "Response has no X-Content-Type-Options: nosniff header", nil)
		}
	}

	// Insecure form: on an HTTPS page, a form posting to an http:// action.
	if https {
		doc.Find("form").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if action, ok := s.Attr("action"); ok && strings.HasPrefix(action, "http://") {
				add(analyze.Warning, "security-insecure-form", "Form submits over insecure http://", map[string]any{"action": action})
				return false
			}
			return true
		})
	}

	return issues
}
