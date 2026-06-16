// Package datalayer audits Google Tag Manager / gtag wiring and the dataLayer event
// stream. It runs two tiers of checks. The static tier reads the page HTML and works in
// any mode: GTM <noscript> fallback, snippet placement, dataLayer pushes before the
// dataLayer is initialized, gtag config IDs with no matching loader, and Consent Mode
// defaults. The runtime tier needs headless rendering (--render headless), which captures
// the post-JS window.dataLayer and the page's network beacons: it inventories events,
// validates GA4 e-commerce events and their parameter types, flags duplicate conversions
// and PII pushed into the dataLayer, and confirms detected tags actually fired a beacon.
package datalayer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// Analyzer audits GTM/gtag configuration and the dataLayer event stream.
type Analyzer struct{}

// New returns a dataLayer analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "datalayer" }
func (Analyzer) Description() string {
	return "GTM/dataLayer audit: snippet wiring, Consent Mode, event inventory, GA4 e-commerce validation, duplicate conversions, and PII (runtime checks need --render headless)"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	var issues []analyze.Issue
	anyRendered := false
	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		sig := scanDOM(p.Doc)
		issues = append(issues, staticChecks(p.FinalURL, sig)...)
		if p.Render != nil && p.Render.Implemented {
			anyRendered = true
			issues = append(issues, runtimeChecks(p.FinalURL, sig, p.Render)...)
		}
	}
	if !anyRendered {
		issues = append(issues, analyze.Issue{
			Analyzer: "datalayer", URL: result.Seed, Severity: analyze.Info,
			Code:    "datalayer-not-collected",
			Message: "dataLayer/event checks require headless rendering. Run with --render headless for event inventory, e-commerce validation, and tag-firing checks.",
		})
	}
	return issues
}

// domSignals are the tag-manager facts pulled from the static HTML.
type domSignals struct {
	gtmIDs       []string // GTM-XXXX container IDs
	gtmInHead    bool     // a GTM container snippet/loader appears within <head>
	gtmNoscript  bool     // a <noscript> GTM ns.html iframe is present
	gtagLoaderID []string // IDs loaded via googletagmanager.com/gtag/js?id= (G-/AW-/GT-)
	gtagConfigID []string // IDs passed to gtag('config', ...)
	hasGA4       bool     // a G- measurement ID is loaded or configured
	hasAds       bool     // an AW- conversion ID is present
	hasMeta      bool     // Meta Pixel (fbq / connect.facebook.net)
	consent      bool     // gtag('consent', 'default'|'update', ...) present
	dlInitIdx    int      // document-order index of first dataLayer initialization (-1 if none)
	dlPushIdx    int      // document-order index of first dataLayer.push (-1 if none)
}

var (
	reGTM        = regexp.MustCompile(`\bGTM-[A-Z0-9]+`)
	reGA4        = regexp.MustCompile(`\bG-[A-Z0-9]{4,}`)
	reAds        = regexp.MustCompile(`\bAW-\d+`)
	reGtagLoader = regexp.MustCompile(`googletagmanager\.com/gtag/js\?id=([A-Za-z0-9-]+)`)
	reGtagConfig = regexp.MustCompile(`gtag\(\s*['"]config['"]\s*,\s*['"]([A-Za-z0-9-]+)['"]`)
	// dataLayer initialization: an explicit assignment, or the canonical GTM-snippet
	// "w[l]=w[l]||[]" form where the array is created lazily.
	reDLInit = regexp.MustCompile(`dataLayer\s*=\s*(\[|\w+\.dataLayer|window\.dataLayer)|\b(\w)\[(\w)\]\s*=\s*\w\[\w\]\s*\|\|\s*\[\]`)
	reDLPush = regexp.MustCompile(`dataLayer\s*\.\s*push\s*\(`)
)

// scanDOM extracts tag-manager signals from the parsed document. Inline scripts are walked
// in document order so push-before-init ordering can be judged.
func scanDOM(doc *goquery.Document) domSignals {
	s := domSignals{dlInitIdx: -1, dlPushIdx: -1}

	// Script src attributes and image/noscript signals, concatenated for substring/ID scans.
	var srcParts []string
	doc.Find("script[src], img[src]").Each(func(_ int, sel *goquery.Selection) {
		if v, ok := sel.Attr("src"); ok {
			srcParts = append(srcParts, v)
		}
	})
	srcBlob := strings.Join(srcParts, "\n")

	// GTM noscript iframe.
	doc.Find("noscript").Each(func(_ int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "googletagmanager.com/ns.html") {
			s.gtmNoscript = true
		}
		if h, ok := sel.Find("iframe").Attr("src"); ok && strings.Contains(h, "googletagmanager.com/ns.html") {
			s.gtmNoscript = true
		}
	})

	// Inline scripts in document order.
	idx := 0
	doc.Find("script:not([src])").Each(func(_ int, sel *goquery.Selection) {
		body := sel.Text()
		if strings.TrimSpace(body) == "" {
			return
		}
		if s.dlInitIdx == -1 && reDLInit.MatchString(body) {
			s.dlInitIdx = idx
		}
		if s.dlPushIdx == -1 && reDLPush.MatchString(body) {
			s.dlPushIdx = idx
		}
		idx++
	})

	// Whole-document text for ID extraction (covers inline scripts and src URLs).
	whole := srcBlob + "\n" + docText(doc)
	s.gtmIDs = dedupe(reGTM.FindAllString(whole, -1))
	s.gtagConfigID = dedupe(captures(reGtagConfig, whole))
	s.gtagLoaderID = dedupe(captures(reGtagLoader, whole))
	for _, id := range append(append([]string{}, s.gtagLoaderID...), s.gtagConfigID...) {
		switch {
		case strings.HasPrefix(id, "G-"):
			s.hasGA4 = true
		case strings.HasPrefix(id, "AW-"):
			s.hasAds = true
		}
	}
	if reGA4.MatchString(whole) {
		s.hasGA4 = true
	}
	if reAds.MatchString(whole) {
		s.hasAds = true
	}
	low := strings.ToLower(whole)
	if strings.Contains(low, "connect.facebook.net") || strings.Contains(low, "fbq(") {
		s.hasMeta = true
	}
	if regexp.MustCompile(`gtag\(\s*['"]consent['"]`).MatchString(whole) {
		s.consent = true
	}

	// Is a GTM snippet/loader inside <head>?
	doc.Find("head script").EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		txt := sel.Text()
		if v, ok := sel.Attr("src"); ok {
			txt += " " + v
		}
		if strings.Contains(txt, "googletagmanager.com/gtm.js") || reGTM.MatchString(txt) {
			s.gtmInHead = true
			return false
		}
		return true
	})
	return s
}

// staticChecks runs the HTML-only tier; these fire in raw and headless mode alike.
func staticChecks(url string, s domSignals) []analyze.Issue {
	var issues []analyze.Issue
	hasGTM := len(s.gtmIDs) > 0
	hasAnyTag := hasGTM || s.hasGA4 || s.hasAds || s.hasMeta || len(s.gtagConfigID) > 0

	if hasGTM {
		if !s.gtmNoscript {
			issues = append(issues, analyze.Issue{
				Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
				Code: "gtm-noscript-missing", Message: "GTM container present but the <noscript> fallback iframe is missing (no tracking for JS-disabled clients)",
				Data: map[string]any{"containers": s.gtmIDs},
			})
		}
		if !s.gtmInHead {
			issues = append(issues, analyze.Issue{
				Analyzer: "datalayer", URL: url, Severity: analyze.Info,
				Code: "gtm-snippet-not-in-head", Message: "GTM container snippet is not in <head>; Google recommends placing it as high in the head as possible",
				Data: map[string]any{"containers": s.gtmIDs},
			})
		}
	}

	// dataLayer.push before the dataLayer is initialized: the early pushes are dropped.
	if s.dlPushIdx != -1 && (s.dlInitIdx == -1 || s.dlPushIdx < s.dlInitIdx) {
		issues = append(issues, analyze.Issue{
			Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
			Code: "datalayer-push-before-init", Message: "dataLayer.push runs before the dataLayer is initialized; those early pushes are lost",
		})
	}

	// Tags present but no dataLayer anywhere in the HTML.
	if (hasGTM || s.hasGA4) && s.dlInitIdx == -1 && s.dlPushIdx == -1 {
		issues = append(issues, analyze.Issue{
			Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
			Code: "datalayer-init-missing", Message: "A tag manager is present but no dataLayer is initialized in the page HTML",
		})
	}

	// gtag('config', X) for an ID that is never loaded by gtag/js and isn't a GTM-managed
	// container. Skip when GTM is present, since GTM can load the tag itself.
	if !hasGTM {
		loaded := setOf(s.gtagLoaderID)
		for _, id := range s.gtagConfigID {
			if !loaded[id] {
				issues = append(issues, analyze.Issue{
					Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
					Code: "gtag-config-id-mismatch", Message: "gtag('config') targets an ID with no matching gtag/js loader on the page",
					Data: map[string]any{"config_id": id, "loaded_ids": s.gtagLoaderID},
				})
			}
		}
	}

	// Consent Mode: present is informational; absent alongside analytics/ads is worth noting.
	if hasAnyTag {
		if s.consent {
			issues = append(issues, analyze.Issue{
				Analyzer: "datalayer", URL: url, Severity: analyze.Info,
				Code: "consent-mode-present", Message: "Google Consent Mode signals detected",
			})
		} else if s.hasGA4 || s.hasAds || hasGTM {
			issues = append(issues, analyze.Issue{
				Analyzer: "datalayer", URL: url, Severity: analyze.Info,
				Code: "consent-mode-missing", Message: "Analytics/ads tags present but no Google Consent Mode default was found",
			})
		}
	}
	return issues
}

// runtimeChecks runs the rendered tier against the captured dataLayer and network beacons.
func runtimeChecks(url string, s domSignals, r *crawler.RenderResult) []analyze.Issue {
	var issues []analyze.Issue
	entries := normalizeEntries(r.DataLayer)

	hasGTM := len(s.gtmIDs) > 0
	if (hasGTM || s.hasGA4) && (!r.DataLayerPresent || len(entries) == 0) {
		issues = append(issues, analyze.Issue{
			Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
			Code: "datalayer-empty", Message: "A tag manager is present but window.dataLayer is empty or absent after rendering",
		})
		// Nothing further to inspect.
		issues = append(issues, tagFiringChecks(url, s, r)...)
		return issues
	}

	// Event inventory.
	names := map[string]int{}
	var order []string
	for _, e := range entries {
		if e.event == "" {
			continue
		}
		if _, seen := names[e.event]; !seen {
			order = append(order, e.event)
		}
		names[e.event]++
	}
	if len(order) > 0 {
		inv := make([]map[string]any, 0, len(order))
		for _, n := range order {
			inv = append(inv, map[string]any{"event": n, "count": names[n]})
		}
		issues = append(issues, analyze.Issue{
			Analyzer: "datalayer", URL: url, Severity: analyze.Info,
			Code: "datalayer-events", Message: "dataLayer event inventory",
			Data: map[string]any{"events": inv},
		})
	}

	// Page-load coverage: GTM always pushes a lifecycle event (gtm.js/gtm.load); a gtag
	// config implies GA4 sends page_view itself. If none of those are present we likely
	// have a page with no measured page view.
	if !hasPageLoad(names, s) && len(entries) > 0 {
		issues = append(issues, analyze.Issue{
			Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
			Code: "page-view-missing", Message: "No page_view or GTM lifecycle event found in the dataLayer",
		})
	}

	// E-commerce validation, parameter typing, and duplicate conversions.
	issues = append(issues, ecommerceChecks(url, entries)...)
	issues = append(issues, duplicateConversionChecks(url, entries)...)
	issues = append(issues, piiChecks(url, entries)...)
	issues = append(issues, tagFiringChecks(url, s, r)...)
	return issues
}

// dlEntry is one normalized dataLayer entry: an event name (when it has one) and its
// parameters, regardless of whether it arrived as a GTM push or a gtag() arguments object.
type dlEntry struct {
	event   string
	params  map[string]any
	command string // gtag command for arguments-style entries: config/consent/js/set/event
}

// normalizeEntries decodes raw dataLayer entries into a uniform shape. GTM pushes are
// objects with an "event" key; gtag() pushes the arguments object, which JSON-serializes
// to {"0":cmd,"1":..,"2":..}.
func normalizeEntries(raw []json.RawMessage) []dlEntry {
	var out []dlEntry
	for _, r := range raw {
		var m map[string]any
		if err := json.Unmarshal(r, &m); err != nil {
			continue // arrays/scalars: not a shape we audit
		}
		e := dlEntry{params: m}
		if ev, ok := m["event"].(string); ok {
			e.event = ev
			out = append(out, e)
			continue
		}
		if cmd, ok := m["0"].(string); ok {
			e.command = cmd
			switch cmd {
			case "event":
				if name, ok := m["1"].(string); ok {
					e.event = name
				}
				if p, ok := m["2"].(map[string]any); ok {
					e.params = p
				}
			}
			out = append(out, e)
			continue
		}
		out = append(out, e)
	}
	return out
}

// requiredParams maps GA4 recommended e-commerce events to the parameters they require.
var requiredParams = map[string][]string{
	"purchase":         {"transaction_id", "value", "currency", "items"},
	"refund":           {"transaction_id", "currency"},
	"add_to_cart":      {"currency", "value", "items"},
	"remove_from_cart": {"currency", "value", "items"},
	"begin_checkout":   {"currency", "value", "items"},
	"add_payment_info": {"currency", "value", "items"},
	"view_item":        {"currency", "value", "items"},
	"view_item_list":   {"items"},
	"select_item":      {"items"},
	"add_to_wishlist":  {"currency", "value", "items"},
}

var reCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

// ecommerceChecks validates required parameters and their types on GA4 e-commerce events.
func ecommerceChecks(url string, entries []dlEntry) []analyze.Issue {
	var issues []analyze.Issue
	for _, e := range entries {
		req, ok := requiredParams[e.event]
		if !ok {
			continue
		}
		// GA4 may nest the payload under "ecommerce".
		params := e.params
		if ec, ok := e.params["ecommerce"].(map[string]any); ok {
			params = mergeMaps(e.params, ec)
		}
		var missing []string
		for _, key := range req {
			if _, present := params[key]; !present {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			issues = append(issues, analyze.Issue{
				Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
				Code: "ecommerce-event-invalid", Message: fmt.Sprintf("%q event is missing required parameters", e.event),
				Data: map[string]any{"event": e.event, "missing": missing},
			})
		}
		// Type checks on the parameters that are present.
		if v, present := params["value"]; present && !isNumber(v) {
			issues = append(issues, typeIssue(url, e.event, "value", "number", v))
		}
		if c, present := params["currency"]; present {
			if cs, ok := c.(string); !ok || !reCurrency.MatchString(cs) {
				issues = append(issues, typeIssue(url, e.event, "currency", "ISO-4217 code", c))
			}
		}
		if items, present := params["items"]; present {
			if arr, ok := items.([]any); !ok || len(arr) == 0 {
				issues = append(issues, analyze.Issue{
					Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
					Code: "datalayer-param-type", Message: fmt.Sprintf("%q event \"items\" should be a non-empty array", e.event),
					Data: map[string]any{"event": e.event, "param": "items"},
				})
			}
		}
	}
	return issues
}

func typeIssue(url, event, param, want string, got any) analyze.Issue {
	return analyze.Issue{
		Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
		Code: "datalayer-param-type", Message: fmt.Sprintf("%q event %q should be a %s", event, param, want),
		Data: map[string]any{"event": event, "param": param, "want": want, "got": fmt.Sprintf("%v", got)},
	}
}

// conversionEvents are events whose double-firing inflates reported conversions.
var conversionEvents = map[string]bool{
	"purchase": true, "generate_lead": true, "sign_up": true,
	"conversion": true, "subscribe": true,
}

// duplicateConversionChecks flags the same conversion event firing more than once, and the
// same purchase transaction_id appearing in multiple purchase events.
func duplicateConversionChecks(url string, entries []dlEntry) []analyze.Issue {
	var issues []analyze.Issue
	counts := map[string]int{}
	txns := map[string]int{}
	for _, e := range entries {
		if conversionEvents[e.event] {
			counts[e.event]++
		}
		if e.event == "purchase" {
			params := e.params
			if ec, ok := e.params["ecommerce"].(map[string]any); ok {
				params = mergeMaps(e.params, ec)
			}
			if tid, ok := params["transaction_id"].(string); ok && tid != "" {
				txns[tid]++
			}
		}
	}
	for name, n := range counts {
		if n > 1 {
			issues = append(issues, analyze.Issue{
				Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
				Code: "duplicate-event", Message: fmt.Sprintf("Conversion event %q fired %d times (risks double-counting)", name, n),
				Data: map[string]any{"event": name, "count": n},
			})
		}
	}
	for tid, n := range txns {
		if n > 1 {
			issues = append(issues, analyze.Issue{
				Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
				Code: "duplicate-transaction", Message: "Same purchase transaction_id fired more than once",
				Data: map[string]any{"transaction_id": tid, "count": n},
			})
		}
	}
	return issues
}

var (
	reEmail  = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`)
	rePhone  = regexp.MustCompile(`\d[\d\s().-]{6,}\d`)
	rePIIKey = regexp.MustCompile(`(?i)\b(phone|tel|mobile|msisdn)\b`)
)

// piiChecks walks every dataLayer entry for personal data pushed into it: email addresses
// anywhere (a strong signal), and phone-number-shaped values under phone-like keys.
func piiChecks(url string, entries []dlEntry) []analyze.Issue {
	seen := map[string]bool{}
	var issues []analyze.Issue
	for _, e := range entries {
		walkStrings(e.params, "", func(key, val string) {
			var kind string
			switch {
			case reEmail.MatchString(val):
				kind = "email"
			case rePIIKey.MatchString(key) && rePhone.MatchString(val):
				kind = "phone"
			default:
				return
			}
			dedupeKey := kind + "|" + key
			if seen[dedupeKey] {
				return
			}
			seen[dedupeKey] = true
			issues = append(issues, analyze.Issue{
				Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
				Code: "datalayer-pii", Message: fmt.Sprintf("Possible %s pushed into the dataLayer (privacy/policy risk)", kind),
				Data: map[string]any{"kind": kind, "key": key, "value": redact(val)},
			})
		})
	}
	return issues
}

// beaconHosts maps a detected tag to the request substrings that prove it fired a beacon.
type beacon struct {
	tag      string
	present  func(domSignals) bool
	patterns []string
}

var beacons = []beacon{
	{tag: "GA4", present: func(s domSignals) bool { return s.hasGA4 }, patterns: []string{"/g/collect", "google-analytics.com/g/", "analytics.google.com/g/"}},
	{tag: "Google Ads", present: func(s domSignals) bool { return s.hasAds }, patterns: []string{"googleadservices.com/pagead/conversion", "google.com/pagead", "googleads.g.doubleclick.net"}},
	{tag: "Meta Pixel", present: func(s domSignals) bool { return s.hasMeta }, patterns: []string{"facebook.com/tr"}},
	{tag: "Google Tag Manager", present: func(s domSignals) bool { return len(s.gtmIDs) > 0 }, patterns: []string{"googletagmanager.com/gtm.js"}},
}

// tagFiringChecks confirms each tag detected in the HTML actually issued a network beacon
// during render. A missing beacon can mean a broken tag, or consent/ad-blocking — the
// message says so rather than asserting a hard failure.
func tagFiringChecks(url string, s domSignals, r *crawler.RenderResult) []analyze.Issue {
	var issues []analyze.Issue
	var fired []string
	for _, b := range beacons {
		if !b.present(s) {
			continue
		}
		if anyContains(r.Requests, b.patterns) {
			fired = append(fired, b.tag)
			continue
		}
		issues = append(issues, analyze.Issue{
			Analyzer: "datalayer", URL: url, Severity: analyze.Warning,
			Code: "tag-not-firing", Message: fmt.Sprintf("%s is installed but issued no network beacon during render (could be a broken tag, consent gating, or ad-blocking)", b.tag),
			Data: map[string]any{"tag": b.tag},
		})
	}
	if len(fired) > 0 {
		sort.Strings(fired)
		issues = append(issues, analyze.Issue{
			Analyzer: "datalayer", URL: url, Severity: analyze.Info,
			Code: "tags-firing", Message: "Tags confirmed firing a network beacon",
			Data: map[string]any{"tags": fired},
		})
	}
	return issues
}

// hasPageLoad reports whether the dataLayer shows the page was measured: an explicit
// page_view, a GTM lifecycle event, or a gtag config (GA4 sends page_view via the tag).
func hasPageLoad(names map[string]int, s domSignals) bool {
	for _, n := range []string{"page_view", "pageview", "gtm.js", "gtm.load", "gtm.dom"} {
		if names[n] > 0 {
			return true
		}
	}
	return len(s.gtagConfigID) > 0
}

// --- small helpers ---

func docText(doc *goquery.Document) string {
	var parts []string
	doc.Find("script:not([src]), noscript").Each(func(_ int, s *goquery.Selection) {
		parts = append(parts, s.Text())
	})
	return strings.Join(parts, "\n")
}

func captures(re *regexp.Regexp, s string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func setOf(in []string) map[string]bool {
	out := make(map[string]bool, len(in))
	for _, s := range in {
		out[s] = true
	}
	return out
}

func isNumber(v any) bool {
	switch v.(type) {
	case float64, int, int64:
		return true
	}
	return false
}

// mergeMaps returns base with inner's keys overlaid; inner (e.g. the "ecommerce" object)
// wins so its value/currency/items are what the type checks read.
func mergeMaps(base, inner map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(inner))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range inner {
		out[k] = v
	}
	return out
}

// walkStrings invokes fn(key, value) for every string leaf in v, recursing through maps
// and slices. The key is the nearest map key (or the parent's for slice elements).
func walkStrings(v any, key string, fn func(key, val string)) {
	switch t := v.(type) {
	case string:
		fn(key, t)
	case map[string]any:
		for k, val := range t {
			walkStrings(val, k, fn)
		}
	case []any:
		for _, val := range t {
			walkStrings(val, key, fn)
		}
	}
}

func redact(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

func anyContains(haystack []string, needles []string) bool {
	for _, h := range haystack {
		for _, n := range needles {
			if strings.Contains(h, n) {
				return true
			}
		}
	}
	return false
}
