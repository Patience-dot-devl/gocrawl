// Package consent audits how a site asks for, and behaves before, visitor consent to
// tracking. It answers two questions a GDPR/ePrivacy review always asks.
//
// **Is consent asked for correctly?** The analyzer detects the consent management platform
// (CMP) in use and inspects the site's Google Consent Mode configuration: whether the v2
// signals Google has required since March 2024 are declared, whether tracking storage is
// granted by default (which defeats the mechanism), whether the defaults are declared before
// the tag loader runs, and whether `wait_for_update` gives an async CMP time to answer first.
//
// **Is it respected?** This is the part a static review misses, and it works because of a
// property of the crawl itself: gocrawl never clicks a consent banner. Every page it fetches
// is therefore a visit by someone who has consented to nothing, and whatever the site sets or
// sends during that visit is its pre-consent behaviour. The analyzer reads two records of it:
//
//   - Cookies. In headless mode the browser's cookie jar is captured after the page settles,
//     which sees JavaScript-set and third-party cookies — where nearly all tracking lives.
//     In raw mode only `Set-Cookie` response headers are visible, so the check still runs but
//     catches only server-set cookies (see the docs for what that misses).
//   - Network beacons. In headless mode the render records outbound requests, so a call to a
//     measurement endpoint before any consent is direct evidence that tags fired early.
//
// The analyzer is passive: it reads what the crawl already fetched and never interacts with a
// banner, clicks "accept", or sends probes of its own.
//
// Consent configuration lives in the shared template, so findings are aggregated and emitted
// once per host rather than repeating on every page. This is an engineering signal, not legal
// advice — whether a given cookie is lawful depends on context the crawler cannot see.
package consent

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// Analyzer audits consent configuration and pre-consent tracking behaviour.
type Analyzer struct{}

// New returns a consent analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "consent" }
func (Analyzer) Description() string {
	return "Consent audit: CMP detection, Google Consent Mode v2 configuration, and tracking cookies/beacons served before consent"
}

// site accumulates one host's consent signals across every page crawled on it.
type site struct {
	host string
	// ref is the URL findings are reported against: the first HTML 200 page on the host.
	ref string
	// pages is every successfully fetched page on the host, in crawl order.
	pages []*crawler.Page

	cmp        string
	cmpFound   bool
	trackers   bool
	mode       consentMode
	modeFound  bool
	loaderLate bool // a consent default was declared after the tag loader on some page
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	var issues []analyze.Issue
	for _, s := range collect(result) {
		issues = append(issues, s.checkConfiguration()...)
		issues = append(issues, s.checkPreConsentCookies()...)
		issues = append(issues, s.checkPreConsentBeacons()...)
	}
	return issues
}

// collect groups pages by host and folds each host's per-page consent signals into one view.
// Hosts are returned in first-seen order so findings are stable across runs.
func collect(result *crawler.Result) []*site {
	var order []string
	byHost := map[string]*site{}

	for _, p := range result.Pages {
		if p == nil || p.StatusCode == 0 || p.FinalURL == "" {
			continue
		}
		u, err := url.Parse(p.FinalURL)
		if err != nil || u.Host == "" {
			continue
		}
		s := byHost[u.Host]
		if s == nil {
			s = &site{host: u.Host}
			byHost[u.Host] = s
			order = append(order, u.Host)
		}
		s.pages = append(s.pages, p)
		s.absorb(p)
	}

	out := make([]*site, 0, len(order))
	for _, h := range order {
		out = append(out, byHost[h])
	}
	return out
}

// absorb folds one page's markup signals into the host's view. Signals are unioned rather
// than overwritten: a CMP or a consent default present on any page counts for the host, which
// is the right reading when a banner is injected by a script that only some pages load.
func (s *site) absorb(p *crawler.Page) {
	if !p.IsHTML() || p.StatusCode != 200 {
		return
	}
	if s.ref == "" {
		s.ref = p.FinalURL
	}

	blob := codeBlob(p.Doc)
	lower := strings.ToLower(blob)

	if !s.cmpFound {
		if name, ok := detectCMP(lower); ok {
			s.cmp, s.cmpFound = name, true
		}
	}
	if hasTrackers(lower) {
		s.trackers = true
	}
	if m := parseConsentMode(blob); m.hasDefault && !s.modeFound {
		s.mode, s.modeFound = m, true
	}
	if s.modeFound && consentDeclaredAfterLoader(p.Doc) {
		s.loaderLate = true
	}
}

// checkConfiguration reports on how consent is asked for: the CMP, and the Consent Mode
// declaration's completeness, defaults, and ordering.
func (s *site) checkConfiguration() []analyze.Issue {
	if s.ref == "" {
		return nil
	}
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "consent", URL: s.ref, Severity: sev, Code: code, Message: msg, Data: data})
	}

	switch {
	case s.cmpFound:
		add(analyze.Info, "consent-cmp-detected",
			fmt.Sprintf("Consent management platform detected: %s", s.cmp),
			map[string]any{"cmp": s.cmp})
	case s.trackers:
		add(analyze.Warning, "consent-no-cmp",
			"Analytics or advertising tags are present but no consent management platform was detected",
			nil)
	}

	if !s.modeFound {
		// Whether Consent Mode is wired at all is the datalayer analyzer's finding
		// (datalayer-consent-mode-missing); this analyzer only judges a configuration it can
		// actually see. A setup driven entirely from inside a GTM container is invisible here.
		return issues
	}

	if s.mode.declaresV1() {
		if missing := s.mode.missingV2Signals(); len(missing) > 0 {
			add(analyze.Warning, "consent-mode-v1-only",
				fmt.Sprintf("Consent Mode declares no %s; Google has required the v2 signals since March 2024", strings.Join(missing, " or ")),
				map[string]any{"missing": missing, "declared": signalNames(s.mode.defaults)})
		}
	}

	if granted := s.mode.grantedByDefault(); len(granted) > 0 && !s.mode.regional {
		add(analyze.Error, "consent-mode-default-granted",
			fmt.Sprintf("Consent Mode grants %s by default, before the visitor has chosen", strings.Join(granted, ", ")),
			map[string]any{"granted": granted})
	}

	if s.mode.waitForUpdate == 0 {
		add(analyze.Info, "consent-mode-no-wait-for-update",
			"Consent Mode sets no wait_for_update, so tags may fire before an asynchronously-loaded CMP delivers the visitor's choice",
			nil)
	}

	if s.loaderLate {
		add(analyze.Warning, "consent-mode-after-tags",
			"The Consent Mode default is declared after the tag loader, so tags can run before the defaults apply",
			nil)
	}

	return issues
}

// signalNames returns the declared signal names, sorted, for a finding's data.
func signalNames(defaults map[string]string) []string {
	out := make([]string, 0, len(defaults))
	for k := range defaults {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// codeBlob concatenates the machine-readable places a CMP or consent configuration shows
// itself: script source URLs, inline script bodies, resource hints, and the class/id
// attributes a banner container carries.
//
// Visible page text is deliberately excluded. Vendor names are matched as substrings, and
// prose mentions them constantly — a CMP vendor's own marketing site lists its competitors by
// name, and a "OneTrust alternative" heading would otherwise be read as an OneTrust install.
// Restricting the surface to code and attributes is what keeps detection precise.
func codeBlob(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	var b strings.Builder
	doc.Find("script").Each(func(_ int, sel *goquery.Selection) {
		if src, ok := sel.Attr("src"); ok {
			b.WriteString(src)
			b.WriteByte('\n')
		}
		b.WriteString(sel.Text())
		b.WriteByte('\n')
	})
	// Resource hints. A CMP loaded at runtime by an application bundle leaves no script tag in
	// the served HTML, but sites still preconnect to it to save the handshake — often the only
	// static trace of a first-party-proxied CMP (e.g. sourcepoint.<site>.com).
	doc.Find("link[href]").Each(func(_ int, sel *goquery.Selection) {
		rel, _ := sel.Attr("rel")
		switch strings.ToLower(strings.TrimSpace(rel)) {
		case "preconnect", "dns-prefetch", "preload", "prefetch":
			href, _ := sel.Attr("href")
			b.WriteString(href)
			b.WriteByte('\n')
		}
	})
	// Banner containers. Several CMPs (Complianz, Borlabs, Real Cookie Banner) are recognisable
	// only from a class or id on the markup they inject, so those attributes are in scope even
	// though the text they wrap is not.
	doc.Find("[class],[id]").Each(func(_ int, sel *goquery.Selection) {
		if v, ok := sel.Attr("class"); ok {
			b.WriteString(v)
			b.WriteByte(' ')
		}
		if v, ok := sel.Attr("id"); ok {
			b.WriteString(v)
			b.WriteByte('\n')
		}
	})
	return b.String()
}

// consentDeclaredAfterLoader reports whether the first Consent Mode default appears later in
// the document than the tag loader it is supposed to precede. Ordering is what makes Consent
// Mode work: defaults declared after gtag.js or gtm.js has already run cannot hold back tags
// that have, by then, already fired.
func consentDeclaredAfterLoader(doc *goquery.Document) bool {
	if doc == nil {
		return false
	}
	consentIdx, loaderIdx := -1, -1
	doc.Find("script").Each(func(i int, sel *goquery.Selection) {
		text := sel.Text()
		if src, ok := sel.Attr("src"); ok {
			text += "\n" + src
		}
		lower := strings.ToLower(text)
		if loaderIdx == -1 && (strings.Contains(lower, "googletagmanager.com/gtag/js") || strings.Contains(lower, "googletagmanager.com/gtm.js")) {
			loaderIdx = i
		}
		if consentIdx == -1 && reConsentCall.MatchString(text) {
			consentIdx = i
		}
	})
	return consentIdx > -1 && loaderIdx > -1 && consentIdx > loaderIdx
}
