// Package tracking detects marketing and analytics tags on a page — Google Tag Manager,
// GA4, Universal Analytics, Google Ads, Meta (Facebook) Pixel, and a few others — and flags
// missing or duplicated installs. It is a SEA analyzer and works from the static HTML, so
// tags injected at runtime by a tag manager are only visible via their container.
package tracking

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// Analyzer scans HTML for known marketing/analytics tags.
type Analyzer struct{}

// New returns a tracking-tag analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "tracking" }
func (Analyzer) Description() string {
	return "Marketing/analytics tag detection: GTM, GA4, Universal Analytics, Google Ads, Meta Pixel; missing and duplicate installs"
}

// GA-version families, used only by the mixed-versions check. Non-GA tags leave family "".
const (
	familyUA  = "ua"
	familyGA4 = "ga4"
)

type signature struct {
	tag     string
	family  string
	signals []string       // lowercased substrings; any match marks the tag present
	idRe    *regexp.Regexp // optional; extracts install IDs (whole match, or capture group 1)
}

// signatures are matched in this (stable) order. Regexes are compiled once at init.
var signatures = []signature{
	{tag: "Google Tag Manager", signals: []string{"googletagmanager.com/gtm.js"}, idRe: regexp.MustCompile(`\bGTM-[A-Z0-9]+`)},
	{tag: "GA4", family: familyGA4, signals: []string{"googletagmanager.com/gtag/js", "gtag/js?id=g-"}, idRe: regexp.MustCompile(`\bG-[A-Z0-9]{4,}`)},
	{tag: "Universal Analytics", family: familyUA, signals: []string{"google-analytics.com/analytics.js", "google-analytics.com/ga.js"}, idRe: regexp.MustCompile(`\bUA-\d+-\d+`)},
	{tag: "Google Ads", signals: nil, idRe: regexp.MustCompile(`\bAW-\d+`)},
	{tag: "Meta Pixel", signals: []string{"connect.facebook.net", "fbq("}, idRe: regexp.MustCompile(`(?:facebook\.com/tr\?id=|fbq\(\s*['"]init['"]\s*,\s*['"])(\d{6,})`)},
	{tag: "LinkedIn Insight", signals: []string{"snap.licdn.com", "_linkedin_partner_id"}, idRe: regexp.MustCompile(`_linkedin_partner_id\s*=\s*["']?(\d+)`)},
	{tag: "Microsoft Bing UET", signals: []string{"bat.bing.com/bat.js", "uetq"}},
	{tag: "TikTok Pixel", signals: []string{"analytics.tiktok.com", "ttq.load", "ttq.page"}},
}

type detection struct {
	tag    string
	family string
	ids    []string
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, a.analyzePage)
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	combined := gather(p.Doc)
	lower := strings.ToLower(combined)

	var detected []detection
	uaPresent, ga4Present := false, false
	for _, sig := range signatures {
		var ids []string
		if sig.idRe != nil {
			for _, m := range sig.idRe.FindAllStringSubmatch(combined, -1) {
				id := m[0]
				if len(m) > 1 && m[1] != "" {
					id = m[1]
				}
				ids = append(ids, id)
			}
		}
		found := len(ids) > 0
		for _, s := range sig.signals {
			if found {
				break
			}
			if strings.Contains(lower, s) {
				found = true
			}
		}
		if !found {
			continue
		}
		ids = dedupe(ids)
		sort.Strings(ids)
		detected = append(detected, detection{tag: sig.tag, family: sig.family, ids: ids})
		switch sig.family {
		case familyUA:
			uaPresent = true
		case familyGA4:
			ga4Present = true
		}
	}

	url := p.FinalURL
	if len(detected) == 0 {
		return []analyze.Issue{{
			Analyzer: "tracking", URL: url, Severity: analyze.Info,
			Code: "tracking-none", Message: "No analytics or marketing tags detected (tags loaded via a tag manager may not be visible in static HTML)",
		}}
	}

	var issues []analyze.Issue
	tags := make([]map[string]any, 0, len(detected))
	for _, d := range detected {
		tags = append(tags, map[string]any{"tag": d.tag, "ids": d.ids})
		if len(d.ids) >= 2 {
			issues = append(issues, analyze.Issue{
				Analyzer: "tracking", URL: url, Severity: analyze.Warning,
				Code: "tracking-duplicate-tag", Message: "Multiple installs of the same tag (risks double-counting)",
				Data: map[string]any{"tag": d.tag, "ids": d.ids, "count": len(d.ids)},
			})
		}
	}
	issues = append(issues, analyze.Issue{
		Analyzer: "tracking", URL: url, Severity: analyze.Info,
		Code: "tracking-tags", Message: "Analytics/marketing tags detected",
		Data: map[string]any{"tags": tags},
	})
	if uaPresent && ga4Present {
		issues = append(issues, analyze.Issue{
			Analyzer: "tracking", URL: url, Severity: analyze.Info,
			Code: "tracking-mixed-ga-versions", Message: "Both Universal Analytics and GA4 are present",
			Data: map[string]any{"ua_ids": idsFor(detected, familyUA), "ga4_ids": idsFor(detected, familyGA4)},
		})
	}
	return issues
}

// gather concatenates the page signals tracking tags hide in: script src attributes, inline
// script bodies, image (pixel) src attributes, and <noscript> contents (which some parsers
// expose as text and others as elements — both are covered).
func gather(doc *goquery.Document) string {
	var parts []string
	doc.Find("script[src], img[src]").Each(func(_ int, s *goquery.Selection) {
		if v, ok := s.Attr("src"); ok {
			parts = append(parts, v)
		}
	})
	doc.Find("script:not([src]), noscript").Each(func(_ int, s *goquery.Selection) {
		parts = append(parts, s.Text())
	})
	return strings.Join(parts, "\n")
}

func idsFor(detected []detection, family string) []string {
	for _, d := range detected {
		if d.family == family {
			return d.ids
		}
	}
	return nil
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
	return out
}
