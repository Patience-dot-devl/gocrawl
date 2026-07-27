package security

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// maxCookieLifetime is the ceiling browsers now enforce on cookie lifetimes (Chrome and
// Safari both cap at 400 days). A Set-Cookie asking for longer is silently truncated, so the
// stated retention period is fiction — which matters for the consent records an SEA setup
// depends on.
const maxCookieLifetime = 400 * 24 * time.Hour

// sensitiveCookieNames are substrings that mark a cookie as carrying session or credential
// state, where a missing HttpOnly turns any XSS into account takeover. Analytics and
// preference cookies are deliberately excluded: they are routinely and legitimately read by
// JavaScript, so flagging them would bury the findings that matter.
var sensitiveCookieNames = []string{"sess", "sid", "auth", "token", "login", "logon", "remember", "jwt", "passwd", "password"}

// auditCookies checks the Set-Cookie attributes this host emits.
//
// Cookies are set by shared middleware, so the same cookie reappears on page after page. The
// audit reports each distinct cookie name once, against the first page that set it.
func (h host) auditCookies() []analyze.Issue {
	var issues []analyze.Issue
	seen := map[string]bool{}

	for _, p := range h.pages {
		if p.Header == nil {
			continue
		}
		secure := isHTTPS(p.FinalURL)
		for _, c := range readSetCookies(p.Header) {
			if seen[c.Name] {
				continue
			}
			seen[c.Name] = true
			issues = append(issues, checkCookie(c, p, secure)...)
		}
	}
	return issues
}

// checkCookie evaluates one cookie's attributes. p is the page that set it, which is what the
// findings are reported against — a cookie is best fixed where it is issued.
func checkCookie(c *http.Cookie, p *crawler.Page, overHTTPS bool) []analyze.Issue {
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		data["cookie"] = c.Name
		issues = append(issues, analyze.Issue{Analyzer: "security", URL: p.FinalURL, Severity: sev, Code: code, Message: msg, Data: data})
	}

	sameSite := sameSiteOf(c)

	if overHTTPS && !c.Secure {
		add(analyze.Error, "security-cookie-no-secure",
			fmt.Sprintf("Cookie %q is set over HTTPS without the Secure attribute", c.Name), map[string]any{})
	}
	if sameSite == "none" && !c.Secure {
		add(analyze.Error, "security-cookie-samesite-none-insecure",
			fmt.Sprintf("Cookie %q declares SameSite=None without Secure, so browsers reject it outright", c.Name), map[string]any{})
	}
	if sameSite == "" {
		add(analyze.Warning, "security-cookie-no-samesite",
			fmt.Sprintf("Cookie %q has no SameSite attribute", c.Name), map[string]any{})
	}
	if !c.HttpOnly && isSensitiveName(c.Name) {
		add(analyze.Warning, "security-cookie-no-httponly",
			fmt.Sprintf("Session-style cookie %q is readable by JavaScript (no HttpOnly)", c.Name), map[string]any{})
	}
	if violation := prefixViolation(c); violation != "" {
		add(analyze.Warning, "security-cookie-prefix-violation",
			fmt.Sprintf("Cookie %q uses a reserved name prefix but %s, so browsers ignore it", c.Name, violation),
			map[string]any{"requirement": violation})
	}
	if life, ok := lifetime(c); ok && life > maxCookieLifetime {
		add(analyze.Info, "security-cookie-long-lived",
			fmt.Sprintf("Cookie %q asks for a %d-day lifetime; browsers cap it at 400 days", c.Name, int(life.Hours()/24)),
			map[string]any{"requested_days": int(life.Hours() / 24)})
	}

	return issues
}

// readSetCookies parses the response's Set-Cookie headers. net/http's parser is reused rather
// than re-implemented so quoted values, expiry formats, and malformed attributes are handled
// the way a browser's would be.
func readSetCookies(hdr http.Header) []*http.Cookie {
	return (&http.Response{Header: hdr}).Cookies()
}

// sameSiteOf returns the cookie's SameSite value lowercased ("lax", "strict", "none"), or ""
// when the attribute is absent. It reads the raw header rather than http.Cookie.SameSite,
// whose zero value does not distinguish "attribute absent" from "attribute unrecognised" —
// and those need different findings.
func sameSiteOf(c *http.Cookie) string {
	for _, attr := range strings.Split(c.Raw, ";") {
		name, value, _ := strings.Cut(attr, "=")
		if strings.EqualFold(strings.TrimSpace(name), "samesite") {
			return strings.ToLower(strings.TrimSpace(value))
		}
	}
	return ""
}

// prefixViolation reports which requirement of a reserved cookie-name prefix the cookie
// fails, or "" when it satisfies them (or uses no prefix). Browsers refuse to store a cookie
// that claims a prefix it doesn't earn, so the cookie silently never arrives.
func prefixViolation(c *http.Cookie) string {
	switch {
	case strings.HasPrefix(c.Name, "__Host-"):
		switch {
		case !c.Secure:
			return "is not Secure"
		case c.Domain != "":
			return "sets a Domain attribute"
		case c.Path != "/":
			return `does not set Path=/`
		}
	case strings.HasPrefix(c.Name, "__Secure-"):
		if !c.Secure {
			return "is not Secure"
		}
	}
	return ""
}

// lifetime returns how long the cookie asks to persist. ok is false for a session cookie
// (neither Max-Age nor Expires) and for an immediate deletion (Max-Age < 0), neither of which
// has a retention period worth checking.
func lifetime(c *http.Cookie) (time.Duration, bool) {
	if c.MaxAge > 0 {
		return time.Duration(c.MaxAge) * time.Second, true
	}
	if c.MaxAge < 0 || c.Expires.IsZero() {
		return 0, false
	}
	return time.Until(c.Expires), true
}

func isSensitiveName(name string) bool {
	lower := strings.ToLower(name)
	for _, s := range sensitiveCookieNames {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func isHTTPS(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && u.Scheme == "https"
}
