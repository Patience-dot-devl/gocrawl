package consent

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
)

// maxExamples bounds how many sample values a finding carries, so an inventory of a
// tag-heavy site stays readable in a report.
const maxExamples = 10

// observedCookie is one cookie seen during the un-consented crawl, with where it came from.
type observedCookie struct {
	name string
	// domain is the cookie's scope when the browser reported it; empty for a cookie read from
	// a Set-Cookie header with no Domain attribute (which scopes it to the serving host).
	domain string
	// thirdParty is true when the cookie's domain is not the crawled host or a parent of it —
	// the shape of a cookie planted by an embedded third-party script.
	thirdParty bool
}

// checkPreConsentCookies reports tracking cookies the site set during a crawl that never
// answered a consent banner.
//
// The evidence differs by render mode, and the finding says which was used: a headless render
// yields the browser's real cookie jar (JavaScript-set and third-party cookies included),
// while a raw crawl can only see Set-Cookie response headers. Most tracking cookies are set
// by JavaScript, so a raw crawl systematically under-reports — the finding's `source` field
// makes that explicit rather than letting a clean result read as a clean site.
func (s *site) checkPreConsentCookies() []analyze.Issue {
	if s.ref == "" {
		return nil
	}
	observed, source := s.observeCookies()
	if len(observed) == 0 {
		return nil
	}

	type flagged struct {
		observedCookie
		tracker trackerCookie
	}
	var trackers []flagged
	var benign []string
	seen := map[string]bool{}

	for _, c := range observed {
		if seen[c.name] {
			continue
		}
		seen[c.name] = true
		if isConsentState(strings.ToLower(c.name)) {
			continue // the CMP's own record of the visitor's choice is strictly necessary
		}
		if t, ok := classifyTracker(c.name); ok {
			trackers = append(trackers, flagged{observedCookie: c, tracker: t})
			continue
		}
		benign = append(benign, c.name)
	}

	sort.Slice(trackers, func(i, j int) bool { return trackers[i].name < trackers[j].name })
	sort.Strings(benign)

	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		data["source"] = source
		issues = append(issues, analyze.Issue{Analyzer: "consent", URL: s.ref, Severity: sev, Code: code, Message: msg, Data: data})
	}

	for _, f := range trackers {
		scope := "first-party"
		if f.thirdParty {
			scope = "third-party"
		}
		add(analyze.Error, "consent-preconsent-tracking-cookie",
			fmt.Sprintf("%s cookie %q (%s, %s) was set before any consent was given", f.tracker.vendor, f.name, f.tracker.purpose, scope),
			map[string]any{
				"cookie":  f.name,
				"vendor":  f.tracker.vendor,
				"purpose": f.tracker.purpose,
				"domain":  f.domain,
				"scope":   scope,
			})
	}

	add(analyze.Info, "consent-cookie-inventory",
		fmt.Sprintf("%d cookie(s) observed before consent: %d classified as tracking, %d other", len(seen), len(trackers), len(benign)),
		map[string]any{
			"total":          len(seen),
			"tracking_count": len(trackers),
			"other_count":    len(benign),
			"other":          cap10(benign),
		})

	return issues
}

// observeCookies returns the cookies seen across the host's pages and the evidence source
// they came from. The browser jar is preferred whenever a render captured one, since it is a
// superset of what response headers can show.
func (s *site) observeCookies() (cookies []observedCookie, source string) {
	for _, p := range s.pages {
		if p.Render == nil || len(p.Render.Cookies) == 0 {
			continue
		}
		for _, c := range p.Render.Cookies {
			cookies = append(cookies, observedCookie{
				name:       c.Name,
				domain:     c.Domain,
				thirdParty: isThirdParty(c.Domain, s.host),
			})
		}
	}
	if len(cookies) > 0 {
		return cookies, "browser cookie jar (headless render)"
	}

	for _, p := range s.pages {
		if p.Header == nil {
			continue
		}
		for _, c := range (&http.Response{Header: p.Header}).Cookies() {
			cookies = append(cookies, observedCookie{
				name:       c.Name,
				domain:     c.Domain,
				thirdParty: isThirdParty(c.Domain, s.host),
			})
		}
	}
	return cookies, "Set-Cookie response headers (raw crawl; JavaScript-set cookies not visible)"
}

// checkPreConsentBeacons reports requests to measurement and advertising endpoints made
// during a render that never answered a consent banner. This is the strongest evidence the
// crawl can produce: a cookie might be argued to be functional, but a call to an analytics
// collector is measurement by definition. Headless mode only — a raw crawl fetches the
// document and never runs the tags that would send them.
func (s *site) checkPreConsentBeacons() []analyze.Issue {
	if s.ref == "" {
		return nil
	}
	hosts := map[string]bool{}
	var examples []string
	rendered := false

	for _, p := range s.pages {
		if p.Render == nil || len(p.Render.Requests) == 0 {
			continue
		}
		rendered = true
		for _, req := range p.Render.Requests {
			h, ok := classifyTrackerRequest(strings.ToLower(req))
			if !ok {
				continue
			}
			if !hosts[h] && len(examples) < maxExamples {
				examples = append(examples, req)
			}
			hosts[h] = true
		}
	}
	if !rendered || len(hosts) == 0 {
		return nil
	}

	names := make([]string, 0, len(hosts))
	for h := range hosts {
		names = append(names, h)
	}
	sort.Strings(names)

	return []analyze.Issue{{
		Analyzer: "consent", URL: s.ref, Severity: analyze.Error,
		Code: "consent-preconsent-tracker-request",
		Message: fmt.Sprintf("%d tracking endpoint(s) were contacted before any consent was given: %s",
			len(names), strings.Join(cap10(names), ", ")),
		Data: map[string]any{"endpoints": names, "examples": examples},
	}}
}

// isThirdParty reports whether a cookie domain sits outside the crawled host's own domain
// tree. A leading dot is the classic wildcard form and is normalised away first.
func isThirdParty(domain, host string) bool {
	if domain == "" {
		return false // no Domain attribute: scoped to the serving host
	}
	d := strings.TrimPrefix(strings.ToLower(domain), ".")
	h := strings.ToLower(host)
	if i := strings.LastIndex(h, ":"); i > -1 {
		h = h[:i] // strip the port
	}
	return h != d && !strings.HasSuffix(h, "."+d)
}

// cap10 truncates a list to maxExamples entries, appending a note when it does, so a report
// never silently implies it listed everything.
func cap10(items []string) []string {
	if len(items) <= maxExamples {
		return items
	}
	out := append([]string{}, items[:maxExamples]...)
	return append(out, fmt.Sprintf("… and %d more", len(items)-maxExamples))
}
