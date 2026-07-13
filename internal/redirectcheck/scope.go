package redirectcheck

import (
	"net/url"
	"strings"
)

// Scope classifies a rule by where its target points, relative to the configured main domain.
type Scope string

const (
	ScopeInScope  Scope = "in-scope"
	ScopeExternal Scope = "external"
	ScopeDynamic  Scope = "dynamic-pattern"
)

// resolveURL turns a rule's Original/Target value into an absolute URL. Relative paths
// ("/foo") resolve against domain over https; absolute values (containing "://") are used
// as given.
func resolveURL(raw, domain string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "://") {
		return raw
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	base := domain
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	return base + raw
}

// isDynamicPattern reports whether a rule uses HubSpot's regex/named-group syntax (Original
// URL) or a {placeholder} substitution (Target) — these can't be resolved to one concrete
// URL without a sample value.
func isDynamicPattern(rule Rule) bool {
	return strings.Contains(rule.Original, "(?P<") ||
		strings.HasPrefix(strings.TrimSpace(rule.Original), "https?://") ||
		strings.Contains(rule.Target, "{")
}

// inScopeHost reports whether host is domain itself or a subdomain of it.
func inScopeHost(host, domain string) bool {
	host = strings.ToLower(host)
	domain = strings.ToLower(domain)
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// Classify determines a rule's Scope. An error is returned only if the target can't be
// parsed as a URL at all.
func Classify(rule Rule, domain string) (Scope, error) {
	if isDynamicPattern(rule) {
		return ScopeDynamic, nil
	}
	u, err := url.Parse(resolveURL(rule.Target, domain))
	if err != nil {
		return "", err
	}
	if inScopeHost(u.Hostname(), domain) {
		return ScopeInScope, nil
	}
	return ScopeExternal, nil
}

// normalizeForSitemap reduces a URL to host+path (dropping scheme, matching sitemap.xml's
// typical canonical-https convention loosely) for membership lookups.
func normalizeForSitemap(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(strings.TrimRight(raw, "/"))
	}
	path := u.Path
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return strings.ToLower(u.Host) + path
}
