// Package security implements security checks: response headers (HSTS, CSP,
// X-Content-Type-Options) and insecure form actions.
package security

import (
	"context"
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// Analyzer performs security header and insecure-form checks.
type Analyzer struct{}

// New returns a new security analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "security" }
func (Analyzer) Description() string {
	return "Security headers (HSTS, CSP, X-Content-Type-Options) and insecure forms"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, a.analyzePage)
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
