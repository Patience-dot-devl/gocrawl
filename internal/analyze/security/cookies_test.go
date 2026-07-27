package security_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
)

// cookieHeaders returns an otherwise-clean header set that additionally sets the given
// cookies, so a cookie test's findings are only about the cookies.
func cookieHeaders(setCookies ...string) http.Header {
	h := secureHeaders()
	for _, c := range setCookies {
		h.Add("Set-Cookie", c)
	}
	return h
}

func TestAuditCookieAttributes(t *testing.T) {
	tests := []struct {
		name      string
		setCookie string
		wantCode  string
		wantSev   analyze.Severity
	}{
		{
			name:      "no Secure over HTTPS",
			setCookie: "session=abc; Path=/; HttpOnly; SameSite=Lax",
			wantCode:  "security-cookie-no-secure",
			wantSev:   analyze.Error,
		},
		{
			name:      "SameSite=None without Secure",
			setCookie: "embed=1; Path=/; SameSite=None",
			wantCode:  "security-cookie-samesite-none-insecure",
			wantSev:   analyze.Error,
		},
		{
			name:      "no SameSite attribute",
			setCookie: "pref=dark; Path=/; Secure",
			wantCode:  "security-cookie-no-samesite",
			wantSev:   analyze.Warning,
		},
		{
			name:      "session cookie readable by JavaScript",
			setCookie: "PHPSESSID=xyz; Path=/; Secure; SameSite=Lax",
			wantCode:  "security-cookie-no-httponly",
			wantSev:   analyze.Warning,
		},
		{
			name:      "__Host- prefix with a Domain attribute",
			setCookie: "__Host-id=1; Path=/; Domain=example.com; Secure; SameSite=Lax; HttpOnly",
			wantCode:  "security-cookie-prefix-violation",
			wantSev:   analyze.Warning,
		},
		{
			name:      "__Secure- prefix without Secure",
			setCookie: "__Secure-id=1; Path=/; SameSite=Lax; HttpOnly",
			wantCode:  "security-cookie-prefix-violation",
			wantSev:   analyze.Warning,
		},
		{
			name:      "lifetime beyond the browser cap",
			setCookie: "_ga=GA1.1; Path=/; Secure; SameSite=Lax; Max-Age=63072000",
			wantCode:  "security-cookie-long-lived",
			wantSev:   analyze.Info,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issues := runAudit(auditPage(t, "https://example.com/", cookieHeaders(tc.setCookie), healthyTLS()))

			got, found := find(issues, tc.wantCode)
			if !found {
				t.Fatalf("expected %q for %q, got %v", tc.wantCode, tc.setCookie, codeList(issues))
			}
			if got.Severity != tc.wantSev {
				t.Errorf("severity = %q, want %q", got.Severity, tc.wantSev)
			}
			if got.Data["cookie"] == nil {
				t.Error("issue data has no cookie name")
			}
		})
	}
}

func TestAuditCleanCookieHasNoFindings(t *testing.T) {
	hdr := cookieHeaders("__Host-session=abc; Path=/; Secure; HttpOnly; SameSite=Lax")
	issues := runAudit(auditPage(t, "https://example.com/", hdr, healthyTLS()))

	for _, is := range issues {
		if strings.HasPrefix(is.Code, "security-cookie-") {
			t.Errorf("unexpected cookie finding %q on a correctly configured cookie: %s", is.Code, is.Message)
		}
	}
}

// TestAuditCookieSecureNotRequiredOverHTTP checks the Secure finding is scoped to HTTPS: on a
// plain-HTTP page a Secure cookie would never be sent back at all, so demanding it there is
// wrong advice. The real problem on that page is the missing HTTPS, reported separately.
func TestAuditCookieSecureNotRequiredOverHTTP(t *testing.T) {
	hdr := http.Header{}
	hdr.Add("Set-Cookie", "session=abc; Path=/; HttpOnly; SameSite=Lax")
	issues := runAudit(auditPage(t, "http://example.com/", hdr, nil))

	if _, found := find(issues, "security-cookie-no-secure"); found {
		t.Error("security-cookie-no-secure emitted for a cookie set over plain HTTP")
	}
	if _, found := find(issues, "security-no-https"); !found {
		t.Error("expected security-no-https for the plain-HTTP page")
	}
}

// TestAuditCookieReportedOncePerName is the deduplication contract: shared middleware sets the
// same cookie on every response, and the report must name the problem once, not per page.
func TestAuditCookieReportedOncePerName(t *testing.T) {
	hdr := cookieHeaders("session=abc; Path=/; HttpOnly; SameSite=Lax")
	issues := runAudit(
		auditPage(t, "https://example.com/", hdr, healthyTLS()),
		auditPage(t, "https://example.com/about", hdr, healthyTLS()),
		auditPage(t, "https://example.com/contact", hdr, healthyTLS()),
	)

	if n := count(issues, "security-cookie-no-secure"); n != 1 {
		t.Errorf("security-cookie-no-secure emitted %d times for one cookie across 3 pages, want 1", n)
	}
}

// TestAuditChecksEveryDistinctCookie confirms deduplication is per name, not per host — a
// response setting several bad cookies must report each of them.
func TestAuditChecksEveryDistinctCookie(t *testing.T) {
	hdr := cookieHeaders(
		"a_session=1; Path=/; HttpOnly; SameSite=Lax",
		"b_session=2; Path=/; HttpOnly; SameSite=Lax",
	)
	issues := runAudit(auditPage(t, "https://example.com/", hdr, healthyTLS()))

	if n := count(issues, "security-cookie-no-secure"); n != 2 {
		t.Errorf("security-cookie-no-secure emitted %d times for 2 insecure cookies, want 2", n)
	}
}

// TestAuditSkipsPagesWithoutHeaders guards the nil-Header path the existing per-page checks
// already tolerate.
func TestAuditSkipsPagesWithoutHeaders(t *testing.T) {
	p := auditPage(t, "https://example.com/", nil, healthyTLS())
	issues := runAudit(p)

	for _, is := range issues {
		if is.Code == "security-cookie-no-secure" {
			t.Error("cookie finding emitted for a page with no captured headers")
		}
	}
	if _, found := find(issues, "security-tls-ok"); !found {
		t.Error("TLS checks should still run when a page carries no headers")
	}
}

// TestAuditIgnoresNonSessionCookieHttpOnly documents the deliberate narrowing of the HttpOnly
// check: analytics and preference cookies are read by JavaScript by design, so flagging them
// would bury the session cookies that matter.
func TestAuditIgnoresNonSessionCookieHttpOnly(t *testing.T) {
	hdr := cookieHeaders("_ga=GA1.1.123; Path=/; Secure; SameSite=Lax")
	issues := runAudit(auditPage(t, "https://example.com/", hdr, healthyTLS()))

	if _, found := find(issues, "security-cookie-no-httponly"); found {
		t.Error("security-cookie-no-httponly emitted for an analytics cookie that JavaScript must read")
	}
}
