package security_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/security"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// healthyTLS is a well-configured handshake: TLS 1.3, a modern cipher, and a chain of leaf +
// intermediate with a year left. Tests mutate a copy of it to isolate one defect at a time.
func healthyTLS() *crawler.TLSInfo {
	return &crawler.TLSInfo{
		Version:     tls.VersionTLS13,
		CipherSuite: tls.TLS_AES_128_GCM_SHA256,
		ALPN:        "h2",
		Chain: []crawler.CertInfo{
			{
				Subject:            "example.com",
				Issuer:             "Example CA R3",
				NotBefore:          time.Now().Add(-30 * 24 * time.Hour),
				NotAfter:           time.Now().Add(365 * 24 * time.Hour),
				SignatureAlgorithm: x509.SHA256WithRSA,
				KeyType:            "ECDSA",
				KeyBits:            256,
			},
			{
				Subject:            "Example CA R3",
				Issuer:             "Example Root",
				NotBefore:          time.Now().Add(-5 * 365 * 24 * time.Hour),
				NotAfter:           time.Now().Add(5 * 365 * 24 * time.Hour),
				SignatureAlgorithm: x509.SHA256WithRSA,
				KeyType:            "RSA",
				KeyBits:            2048,
				IsCA:               true,
			},
		},
	}
}

// secureHeaders is a response header set that passes every audit header check, so a test can
// introduce exactly one problem.
func secureHeaders() http.Header {
	h := http.Header{}
	h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	return h
}

// auditPage builds an HTML 200 page at url with the given headers and handshake.
func auditPage(t *testing.T, url string, hdr http.Header, tlsInfo *crawler.TLSInfo) *crawler.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<html><body>hello</body></html>"))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return &crawler.Page{
		RequestedURL: url,
		FinalURL:     url,
		StatusCode:   200,
		ContentType:  "text/html",
		Doc:          doc,
		Header:       hdr,
		TLS:          tlsInfo,
	}
}

// runAudit analyzes the pages with the audit enabled and returns the resulting issues.
func runAudit(pages ...*crawler.Page) []analyze.Issue {
	res := &crawler.Result{Pages: pages}
	return security.New(security.WithAudit(true)).Analyze(context.Background(), res)
}

// find returns the first issue with the given code, and whether one exists.
func find(issues []analyze.Issue, code string) (analyze.Issue, bool) {
	for _, is := range issues {
		if is.Code == code {
			return is, true
		}
	}
	return analyze.Issue{}, false
}

// count returns how many issues carry the given code.
func count(issues []analyze.Issue, code string) int {
	n := 0
	for _, is := range issues {
		if is.Code == code {
			n++
		}
	}
	return n
}

func TestAuditOffByDefault(t *testing.T) {
	p := auditPage(t, "http://example.com/", http.Header{}, nil)
	got := codes(security.New().Analyze(context.Background(), &crawler.Result{Pages: []*crawler.Page{p}}))

	for _, unwanted := range []string{"security-no-https", "security-tls-ok", "security-cookie-no-secure", "security-missing-frame-protection"} {
		if got[unwanted] {
			t.Errorf("audit-only issue %q emitted without WithAudit", unwanted)
		}
	}
}

func TestAuditHealthyTLSReportsOK(t *testing.T) {
	issues := runAudit(auditPage(t, "https://example.com/", secureHeaders(), healthyTLS()))

	ok, found := find(issues, "security-tls-ok")
	if !found {
		t.Fatalf("expected security-tls-ok on a healthy handshake, got %v", codeList(issues))
	}
	if ok.Severity != analyze.Info {
		t.Errorf("security-tls-ok severity = %q, want info", ok.Severity)
	}
	if got := ok.Data["version"]; got != "TLS 1.3" {
		t.Errorf("data version = %v, want TLS 1.3", got)
	}
	if days, _ := ok.Data["days_remaining"].(int); days < 360 {
		t.Errorf("data days_remaining = %v, want roughly 365", ok.Data["days_remaining"])
	}
	for _, unwanted := range []string{"security-tls-obsolete-version", "security-tls-weak-cipher", "security-tls-incomplete-chain", "security-no-https"} {
		if _, bad := find(issues, unwanted); bad {
			t.Errorf("unexpected issue %q on a healthy handshake", unwanted)
		}
	}
}

func TestAuditCertificateExpiry(t *testing.T) {
	tests := []struct {
		name     string
		expires  time.Duration
		wantCode string
		wantSev  analyze.Severity
	}{
		{"expired", -2 * 24 * time.Hour, "security-tls-cert-expired", analyze.Error},
		{"under 14 days", 5 * 24 * time.Hour, "security-tls-cert-expiring-soon", analyze.Error},
		{"under 30 days", 20 * 24 * time.Hour, "security-tls-cert-expiring-soon", analyze.Warning},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := healthyTLS()
			info.Chain[0].NotAfter = time.Now().Add(tc.expires)
			issues := runAudit(auditPage(t, "https://example.com/", secureHeaders(), info))

			got, found := find(issues, tc.wantCode)
			if !found {
				t.Fatalf("expected %q, got %v", tc.wantCode, codeList(issues))
			}
			if got.Severity != tc.wantSev {
				t.Errorf("severity = %q, want %q", got.Severity, tc.wantSev)
			}
			if _, healthy := find(issues, "security-tls-ok"); healthy {
				t.Error("security-tls-ok emitted alongside an expiry finding")
			}
		})
	}
}

func TestAuditCertificateAndProtocolDefects(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*crawler.TLSInfo)
		wantCode string
		wantSev  analyze.Severity
	}{
		{"obsolete protocol", func(i *crawler.TLSInfo) { i.Version = tls.VersionTLS10 }, "security-tls-obsolete-version", analyze.Error},
		{"TLS 1.2", func(i *crawler.TLSInfo) { i.Version = tls.VersionTLS12 }, "security-tls-legacy-version", analyze.Info},
		{"insecure cipher", func(i *crawler.TLSInfo) {
			i.Version = tls.VersionTLS12
			i.CipherSuite = tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA
		}, "security-tls-weak-cipher", analyze.Error},
		{"not yet valid", func(i *crawler.TLSInfo) { i.Chain[0].NotBefore = time.Now().Add(48 * time.Hour) }, "security-tls-cert-not-yet-valid", analyze.Error},
		{"self-signed leaf", func(i *crawler.TLSInfo) {
			i.Chain = i.Chain[:1]
			i.Chain[0].SelfSigned = true
			i.Chain[0].Issuer = i.Chain[0].Subject
		}, "security-tls-cert-self-signed", analyze.Error},
		{"missing intermediate", func(i *crawler.TLSInfo) { i.Chain = i.Chain[:1] }, "security-tls-incomplete-chain", analyze.Warning},
		{"SHA-1 leaf", func(i *crawler.TLSInfo) { i.Chain[0].SignatureAlgorithm = x509.SHA1WithRSA }, "security-tls-weak-signature", analyze.Error},
		{"1024-bit RSA leaf", func(i *crawler.TLSInfo) { i.Chain[0].KeyType, i.Chain[0].KeyBits = "RSA", 1024 }, "security-tls-weak-key", analyze.Error},
		{"P-192 leaf", func(i *crawler.TLSInfo) { i.Chain[0].KeyBits = 192 }, "security-tls-weak-key", analyze.Error},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := healthyTLS()
			tc.mutate(info)
			issues := runAudit(auditPage(t, "https://example.com/", secureHeaders(), info))

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

// TestAuditExemptsSelfSignedRootFromSignatureCheck guards the carve-out for trust anchors: a
// root is trusted by identity, not by its own signature, and SHA-1 roots are still ordinary
// in trust stores. Flagging them would be noise on otherwise perfect configurations.
func TestAuditExemptsSelfSignedRootFromSignatureCheck(t *testing.T) {
	info := healthyTLS()
	info.Chain = append(info.Chain, crawler.CertInfo{
		Subject:            "Example Root",
		Issuer:             "Example Root",
		NotAfter:           time.Now().Add(10 * 365 * 24 * time.Hour),
		SignatureAlgorithm: x509.SHA1WithRSA,
		KeyType:            "RSA",
		KeyBits:            4096,
		IsCA:               true,
		SelfSigned:         true,
	})
	issues := runAudit(auditPage(t, "https://example.com/", secureHeaders(), info))

	if _, found := find(issues, "security-tls-weak-signature"); found {
		t.Error("SHA-1 self-signed root flagged; roots are trusted by identity, not signature")
	}
}

// TestAuditReportsOncePerHost is the aggregation contract: TLS state is server configuration,
// so a 3-page crawl of one host must not produce the same finding three times.
func TestAuditReportsOncePerHost(t *testing.T) {
	info := healthyTLS()
	info.Chain = info.Chain[:1] // incomplete chain, on every page
	issues := runAudit(
		auditPage(t, "https://example.com/", secureHeaders(), info),
		auditPage(t, "https://example.com/about", secureHeaders(), info),
		auditPage(t, "https://example.com/contact", secureHeaders(), info),
	)

	if n := count(issues, "security-tls-incomplete-chain"); n != 1 {
		t.Errorf("security-tls-incomplete-chain emitted %d times across 3 pages of one host, want 1", n)
	}
}

// TestAuditReportsPerHostSeparately is the other half of that contract: two hosts have two
// independent configurations and must each be reported.
func TestAuditReportsPerHostSeparately(t *testing.T) {
	weak := healthyTLS()
	weak.Chain = weak.Chain[:1]
	issues := runAudit(
		auditPage(t, "https://example.com/", secureHeaders(), healthyTLS()),
		auditPage(t, "https://shop.example.com/", secureHeaders(), weak),
	)

	got, found := find(issues, "security-tls-incomplete-chain")
	if !found {
		t.Fatalf("expected the second host's chain finding, got %v", codeList(issues))
	}
	if got.URL != "https://shop.example.com/" {
		t.Errorf("issue URL = %q, want it reported against the host that has the defect", got.URL)
	}
}

func TestAuditFlagsPlainHTTP(t *testing.T) {
	issues := runAudit(
		auditPage(t, "http://example.com/", http.Header{}, nil),
		auditPage(t, "http://example.com/about", http.Header{}, nil),
	)

	got, found := find(issues, "security-no-https")
	if !found {
		t.Fatalf("expected security-no-https, got %v", codeList(issues))
	}
	if got.Severity != analyze.Error {
		t.Errorf("severity = %q, want error", got.Severity)
	}
	if n, _ := got.Data["count"].(int); n != 2 {
		t.Errorf("data count = %v, want 2 plaintext pages", got.Data["count"])
	}
}

// TestAuditSkipsTLSChecksWithoutHandshake covers headless render mode, where responses never
// pass through Go's TLS stack: the audit must stay silent rather than invent findings.
func TestAuditSkipsTLSChecksWithoutHandshake(t *testing.T) {
	issues := runAudit(auditPage(t, "https://example.com/", secureHeaders(), nil))

	for _, code := range []string{"security-tls-ok", "security-tls-incomplete-chain", "security-no-https"} {
		if _, found := find(issues, code); found {
			t.Errorf("emitted %q with no captured handshake", code)
		}
	}
}

// codeList renders the codes present, for readable failure messages.
func codeList(issues []analyze.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, is := range issues {
		out = append(out, is.Code)
	}
	return out
}
