package crawler

import (
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// normalizeURL canonicalizes a URL for deduplication and lookup: it lowercases the scheme
// and host, drops the fragment, removes the default port, and trims a trailing slash from
// non-root paths. Query strings are preserved. Invalid URLs are returned unchanged.
func normalizeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""

	// Strip default ports.
	if (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) ||
		(u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) {
		u.Host = u.Host[:strings.LastIndex(u.Host, ":")]
	}

	if len(u.Path) > 1 {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

// resolveURL resolves href relative to base and returns the absolute, normalized form.
// It returns ("", false) for unusable links (fragments, mailto:, javascript:, etc.).
func resolveURL(base *url.URL, href string) (string, bool) {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return "", false
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	abs := base.ResolveReference(ref)
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return "", false
	}
	return normalizeURL(abs.String()), true
}

// registrableDomain returns the eTLD+1 (e.g. "example.co.uk") for a host, falling back to
// the host itself when it cannot be determined.
func registrableDomain(host string) string {
	host = stripPort(host)
	d, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return host
	}
	return d
}

func stripPort(host string) string {
	if i := strings.LastIndex(host, ":"); i >= 0 {
		return host[:i]
	}
	return host
}

// sameSite reports whether host is in scope relative to seedHost, honoring the
// allowSubdomains setting.
func sameSite(seedHost, host string, allowSubdomains bool) bool {
	seedHost = strings.ToLower(stripPort(seedHost))
	host = strings.ToLower(stripPort(host))
	if seedHost == host {
		return true
	}
	if !allowSubdomains {
		return false
	}
	return registrableDomain(seedHost) == registrableDomain(host)
}
