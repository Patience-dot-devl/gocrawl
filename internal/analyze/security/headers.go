package security

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// hstsFloor is the shortest Strict-Transport-Security max-age worth having. Six months is the
// figure the HSTS preload list requires (it asks for a year, and rejects anything under 18
// weeks); below it, a user who hasn't visited recently is unprotected again.
const hstsFloor = 180 * 24 * 60 * 60

// versionRe matches a version number in a Server or X-Powered-By value ("nginx/1.18.0",
// "PHP/8.1.2"). A bare product name is not a finding — knowing the software is unavoidable;
// publishing its patch level is what hands an attacker a CVE shortlist.
var versionRe = regexp.MustCompile(`\d+\.\d+`)

// maxAgeRe pulls the max-age directive out of a Strict-Transport-Security value.
var maxAgeRe = regexp.MustCompile(`(?i)\bmax-age\s*=\s*"?(\d+)`)

// auditHeaders runs the deeper response-header policy checks. The three baseline header
// checks (HSTS/CSP/nosniff presence) stay in the per-page pass; this covers the quality of
// what is there and the policies the baseline doesn't look for.
//
// Response headers come from server or CDN configuration, so they are checked once per host
// against a representative page rather than repeated across the crawl.
func (h host) auditHeaders() []analyze.Issue {
	p := h.representative()
	if p == nil {
		return nil
	}
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "security", URL: p.FinalURL, Severity: sev, Code: code, Message: msg, Data: data})
	}

	hdr := p.Header
	if hsts := hdr.Get("Strict-Transport-Security"); hsts != "" {
		if age, ok := hstsMaxAge(hsts); ok && age < hstsFloor {
			add(analyze.Warning, "security-hsts-short-max-age",
				fmt.Sprintf("Strict-Transport-Security max-age is %d seconds (%d days), below the 180-day floor", age, age/86400),
				map[string]any{"max_age": age})
		}
		if !strings.Contains(strings.ToLower(hsts), "includesubdomains") {
			add(analyze.Info, "security-hsts-no-subdomains",
				"Strict-Transport-Security does not set includeSubDomains",
				map[string]any{"value": hsts})
		}
	}

	if hdr.Get("Referrer-Policy") == "" && !hasMetaReferrer(p) {
		add(analyze.Info, "security-missing-referrer-policy",
			"Response sets no Referrer-Policy header", nil)
	}

	if hdr.Get("X-Frame-Options") == "" && !strings.Contains(strings.ToLower(hdr.Get("Content-Security-Policy")), "frame-ancestors") {
		add(analyze.Warning, "security-missing-frame-protection",
			"Response has neither X-Frame-Options nor a CSP frame-ancestors directive", nil)
	}

	for _, name := range []string{"Server", "X-Powered-By", "X-AspNet-Version", "X-Generator"} {
		if v := hdr.Get(name); v != "" && versionRe.MatchString(v) {
			add(analyze.Info, "security-version-disclosure",
				fmt.Sprintf("%s header discloses a software version: %s", name, v),
				map[string]any{"header": name, "value": v})
		}
	}

	return issues
}

// representative returns the page whose headers stand in for the host: the first HTML 200
// that actually carried response headers. A redirect or error response is a poor sample —
// many stacks attach the full security-header set only to real page responses.
func (h host) representative() *crawler.Page {
	for _, p := range h.pages {
		if p.StatusCode == http.StatusOK && p.IsHTML() && p.Header != nil {
			return p
		}
	}
	return nil
}

// hasMetaReferrer reports whether the document sets a referrer policy in markup instead of a
// header, which is equally valid and shouldn't be reported as missing.
func hasMetaReferrer(p *crawler.Page) bool {
	if p.Doc == nil {
		return false
	}
	return p.Doc.Find(`meta[name="referrer"]`).Length() > 0
}

// hstsMaxAge extracts the max-age directive in seconds. ok is false when the header carries
// no parseable max-age, in which case browsers ignore the policy entirely — a case the
// baseline presence check already covers as "header present", so it isn't re-flagged here.
func hstsMaxAge(value string) (int, bool) {
	m := maxAgeRe.FindStringSubmatch(value)
	if m == nil {
		return 0, false
	}
	age, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return age, true
}
