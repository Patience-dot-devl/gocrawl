package security_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

func TestAuditHeaderPolicy(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(http.Header)
		wantCode string
		wantSev  analyze.Severity
	}{
		{
			name:     "HSTS max-age under six months",
			mutate:   func(h http.Header) { h.Set("Strict-Transport-Security", "max-age=86400; includeSubDomains") },
			wantCode: "security-hsts-short-max-age",
			wantSev:  analyze.Warning,
		},
		{
			name:     "HSTS without includeSubDomains",
			mutate:   func(h http.Header) { h.Set("Strict-Transport-Security", "max-age=31536000") },
			wantCode: "security-hsts-no-subdomains",
			wantSev:  analyze.Info,
		},
		{
			name:     "no referrer policy",
			mutate:   func(h http.Header) { h.Del("Referrer-Policy") },
			wantCode: "security-missing-referrer-policy",
			wantSev:  analyze.Info,
		},
		{
			name:     "no framing protection",
			mutate:   func(h http.Header) { h.Set("Content-Security-Policy", "default-src 'self'") },
			wantCode: "security-missing-frame-protection",
			wantSev:  analyze.Warning,
		},
		{
			name:     "Server header discloses a version",
			mutate:   func(h http.Header) { h.Set("Server", "nginx/1.18.0") },
			wantCode: "security-version-disclosure",
			wantSev:  analyze.Info,
		},
		{
			name:     "X-Powered-By discloses a version",
			mutate:   func(h http.Header) { h.Set("X-Powered-By", "PHP/8.1.2") },
			wantCode: "security-version-disclosure",
			wantSev:  analyze.Info,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hdr := secureHeaders()
			tc.mutate(hdr)
			issues := runAudit(auditPage(t, "https://example.com/", hdr, healthyTLS()))

			got, found := find(issues, tc.wantCode)
			if !found {
				t.Fatalf("expected %q, got %v", tc.wantCode, codeList(issues))
			}
			if got.Severity != tc.wantSev {
				t.Errorf("severity = %q, want %q", got.Severity, tc.wantSev)
			}
		})
	}
}

func TestAuditCleanHeadersHaveNoFindings(t *testing.T) {
	issues := runAudit(auditPage(t, "https://example.com/", secureHeaders(), healthyTLS()))

	unwanted := []string{
		"security-hsts-short-max-age", "security-hsts-no-subdomains",
		"security-missing-referrer-policy", "security-missing-frame-protection",
		"security-version-disclosure",
	}
	for _, code := range unwanted {
		if got, found := find(issues, code); found {
			t.Errorf("unexpected %q on a well-configured response: %s", code, got.Message)
		}
	}
}

// TestAuditAcceptsFrameAncestorsInsteadOfXFrameOptions covers the modern replacement: a CSP
// frame-ancestors directive is framing protection, so demanding X-Frame-Options too would be
// obsolete advice.
func TestAuditAcceptsFrameAncestorsInsteadOfXFrameOptions(t *testing.T) {
	hdr := secureHeaders()
	hdr.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
	hdr.Del("X-Frame-Options")
	issues := runAudit(auditPage(t, "https://example.com/", hdr, healthyTLS()))

	if _, found := find(issues, "security-missing-frame-protection"); found {
		t.Error("flagged missing framing protection despite a CSP frame-ancestors directive")
	}
}

// TestAuditAcceptsMetaReferrer covers the markup form of the referrer policy, which is as
// valid as the header.
func TestAuditAcceptsMetaReferrer(t *testing.T) {
	hdr := secureHeaders()
	hdr.Del("Referrer-Policy")
	p := auditPage(t, "https://example.com/", hdr, healthyTLS())
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(
		`<html><head><meta name="referrer" content="strict-origin"></head><body>hi</body></html>`))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	p.Doc = doc

	if _, found := find(runAudit(p), "security-missing-referrer-policy"); found {
		t.Error("flagged a missing referrer policy despite a <meta name=\"referrer\"> declaration")
	}
}

// TestAuditIgnoresVersionlessServerHeader keeps the disclosure check to what is actionable:
// the product name alone can't be hidden and isn't a finding.
func TestAuditIgnoresVersionlessServerHeader(t *testing.T) {
	hdr := secureHeaders()
	hdr.Set("Server", "cloudflare")
	issues := runAudit(auditPage(t, "https://example.com/", hdr, healthyTLS()))

	if _, found := find(issues, "security-version-disclosure"); found {
		t.Error("flagged a Server header that discloses no version")
	}
}

// TestAuditHeadersUseRealPageResponse checks the representative page is a genuine HTML 200:
// many stacks attach the full security-header set only to real page responses, so sampling a
// redirect or error would report false gaps.
func TestAuditHeadersUseRealPageResponse(t *testing.T) {
	bare := &crawler.Page{
		RequestedURL: "https://example.com/gone",
		FinalURL:     "https://example.com/gone",
		StatusCode:   404,
		Header:       http.Header{},
		TLS:          healthyTLS(),
	}
	good := auditPage(t, "https://example.com/", secureHeaders(), healthyTLS())
	issues := runAudit(bare, good)

	if got, found := find(issues, "security-missing-frame-protection"); found {
		t.Errorf("sampled the 404's headers instead of the HTML 200's: %s", got.Message)
	}
}
