package crawler

import (
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// normalizeURL canonicalizes a URL for deduplication and lookup: it lowercases the scheme
// and host, drops the fragment, removes the default port, and trims a trailing slash from
// non-root paths. When stripQuery is true the query string is dropped as well, so URLs that
// differ only by their query (e.g. ?page=1 vs ?page=2) collapse to a single entry; otherwise
// the query is preserved. Invalid URLs are returned unchanged.
func normalizeURL(raw string, stripQuery bool) string {
	return canonicalURL(raw, stripQuery, true)
}

// normalizeURLKeepSlash canonicalizes like normalizeURL but preserves a non-root trailing
// slash. It exists so callers can compare a link's authored target against a redirect's
// destination without the trailing-slash strip — applied by normalizeURL for index dedup —
// hiding the difference: with the slash kept, "/contact/" and "/contact" stay distinct.
func normalizeURLKeepSlash(raw string, stripQuery bool) string {
	return canonicalURL(raw, stripQuery, false)
}

// canonicalURL is the shared normalizer behind normalizeURL (trimSlash=true) and
// normalizeURLKeepSlash (trimSlash=false). Invalid URLs are returned unchanged.
func canonicalURL(raw string, stripQuery, trimSlash bool) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	// Credentials should never survive normalization: they're routed through
	// Options.BasicAuthUser/Pass at seed intake (see SanitizeSeed), and any that arrive by
	// another path (e.g. a redirect Location header) must not round-trip into the URL index
	// or reports.
	u.User = nil
	if stripQuery {
		u.RawQuery = ""
		u.ForceQuery = false
	}

	// Strip default ports.
	if (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) ||
		(u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) {
		u.Host = u.Host[:strings.LastIndex(u.Host, ":")]
	}

	if trimSlash && len(u.Path) > 1 {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

// sameTarget reports whether two URLs are the same address once normalized with the trailing
// slash preserved. It is how LinkPointsToRedirect tells a genuine redirect (scheme, host, or
// path change) from one that only differs by the trailing slash that index normalization
// strips — so a link authored with the site's canonical trailing slash isn't mistaken for
// pointing at a redirect.
func sameTarget(a, b string, stripQuery bool) bool {
	return normalizeURLKeepSlash(a, stripQuery) == normalizeURLKeepSlash(b, stripQuery)
}

// LinkPointsToRedirect reports whether an internal link whose resolved (slash-preserving)
// target is `resolved` points at `target`, a crawled page that redirects to a genuinely
// different URL. It returns false when the page's only redirect is the trailing slash the
// crawl index strips before fetching, so a link authored with the site's canonical trailing
// slash (e.g. WordPress permalinks) is not reported as pointing at a redirect.
func (r *Result) LinkPointsToRedirect(resolved string, target *Page) bool {
	return len(target.Redirects) > 0 && !sameTarget(resolved, target.FinalURL, r.Opts.StripQuery)
}

// resolveURL resolves href relative to base and returns two normalized absolute forms: key is
// the dedup/lookup form (trailing slash stripped, matching the crawl index) and resolved
// preserves the trailing slash so callers can distinguish a genuinely-redirecting target from
// one that differs only by the slash the index strips. It returns ("", "", false) for
// unusable links (fragments, mailto:, javascript:, etc.).
func resolveURL(base *url.URL, href string, stripQuery bool) (key, resolved string, ok bool) {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return "", "", false
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", "", false
	}
	abs := base.ResolveReference(ref)
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return "", "", false
	}
	s := abs.String()
	return normalizeURL(s, stripQuery), normalizeURLKeepSlash(s, stripQuery), true
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
