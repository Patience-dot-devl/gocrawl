// Package landing assesses paid-campaign landing pages: how well the page's title and
// headings reflect the campaign keywords (utm_term / utm_campaign / utm_content), plus the
// quality-score signals ads care about (title, H1, meta description, indexability, HTTPS).
//
// A page is treated as a landing page when its own URL carries campaign UTM parameters, or
// when another crawled page links to it with such parameters. The campaign keywords are
// derived entirely from the crawl's own link data — no external campaign feed is required.
// Because external destinations are usually not crawled, the page-level checks fire for the
// internally-reachable and self-tagged landing pages the crawl actually fetched.
package landing

import (
	"context"
	"math"
	"net/url"
	"strings"
	"unicode"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/seaurl"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// campaignKeys are the UTM parameters that carry keyword/intent signal.
var campaignKeys = []string{"utm_term", "utm_campaign", "utm_content"}

// Coverage thresholds for keyword alignment between campaign terms and the page's strong
// signals (title + H1/H2).
const (
	mismatchBelow = 0.2
	weakBelow     = 0.5
)

// Analyzer scores landing-page relevance for ad destinations.
type Analyzer struct{}

// New returns a landing-page analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "landing" }
func (Analyzer) Description() string {
	return "Landing-page relevance: campaign-keyword alignment with title/headings, plus indexability/HTTPS/title/H1 quality signals"
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	campaign := buildCampaignMap(result)
	return analyze.EachPage(result, func(p *crawler.Page) []analyze.Issue {
		return analyzePage(p, campaign)
	})
}

// buildCampaignMap collects campaign keyword tokens keyed by normalized URL, from every
// page's own URL and from every outbound link's UTM parameters.
func buildCampaignMap(result *crawler.Result) map[string][]string {
	m := map[string][]string{}
	add := func(rawURL string) {
		u := seaurl.Parse(rawURL)
		if !u.Tagged() {
			return
		}
		var terms []string
		for _, k := range campaignKeys {
			if v, ok := u.Values[k]; ok {
				terms = append(terms, tokenize(v)...)
			}
		}
		if len(terms) > 0 {
			key := normalizeKey(rawURL)
			m[key] = append(m[key], terms...)
		}
	}
	for _, p := range result.Pages {
		add(p.FinalURL)
		for _, link := range p.Links {
			add(link.URL)
		}
	}
	for k := range m {
		m[k] = dedupe(m[k])
	}
	return m
}

func analyzePage(p *crawler.Page, campaign map[string][]string) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	terms := campaign[normalizeKey(p.FinalURL)]
	if len(terms) == 0 {
		return nil // not a landing page
	}

	doc := p.Doc
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "landing", URL: p.FinalURL, Severity: sev, Code: code, Message: msg, Data: data})
	}

	// Ad quality-score signals.
	if !strings.HasPrefix(strings.ToLower(p.FinalURL), "https://") {
		add(analyze.Warning, "landing-not-https", "Paid landing page is not served over HTTPS", nil)
	}
	if robots, ok := metaContent(doc, "robots"); ok && strings.Contains(strings.ToLower(robots), "noindex") {
		add(analyze.Error, "landing-noindex", "Paid landing page is marked noindex", map[string]any{"robots": robots})
	}
	title := strings.TrimSpace(doc.Find("head title").First().Text())
	if title == "" {
		add(analyze.Warning, "landing-missing-title", "Landing page has no <title>", nil)
	}
	if doc.Find("h1").Length() == 0 {
		add(analyze.Warning, "landing-missing-h1", "Landing page has no <h1>", nil)
	}
	if desc, ok := metaContent(doc, "description"); !ok || strings.TrimSpace(desc) == "" {
		add(analyze.Info, "landing-missing-description", "Landing page has no meta description", nil)
	}

	// Keyword alignment: campaign terms vs the page's strong signals (title + H1/H2).
	strong := map[string]bool{}
	for _, tok := range tokenize(title) {
		strong[tok] = true
	}
	doc.Find("h1, h2").Each(func(_ int, s *goquery.Selection) {
		for _, tok := range tokenize(s.Text()) {
			strong[tok] = true
		}
	})
	var matched, missing []string
	for _, tok := range terms {
		if strong[tok] {
			matched = append(matched, tok)
		} else {
			missing = append(missing, tok)
		}
	}
	coverage := float64(len(matched)) / float64(len(terms))
	data := map[string]any{"campaign_terms": terms, "matched": matched, "missing": missing, "coverage": round2(coverage)}
	switch {
	case coverage < mismatchBelow:
		add(analyze.Warning, "landing-keyword-mismatch", "Campaign keywords are absent from the landing page title/headings", data)
	case coverage < weakBelow:
		add(analyze.Info, "landing-keyword-weak", "Campaign keywords are only weakly reflected in the landing page title/headings", data)
	default:
		add(analyze.Info, "landing-keyword-aligned", "Campaign keywords align with the landing page title/headings", data)
	}
	return issues
}

// normalizeKey reduces a URL to a host+path key (lowercased, trailing slash trimmed, query
// and fragment dropped) so a UTM-tagged link and its crawled destination map together. It is
// used consistently on both sides; it deliberately does not depend on the crawler's own
// (unexported) normalization.
func normalizeKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(strings.TrimRight(raw, "/"))
	}
	host := strings.ToLower(u.Host)
	path := strings.TrimRight(u.Path, "/")
	if path == "" {
		path = "/"
	}
	return host + path
}

// tokenize lowercases, splits on non-alphanumeric runes, and drops short tokens and common
// stopwords, returning unique tokens in first-seen order.
func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	seen := map[string]bool{}
	var out []string
	for _, f := range fields {
		if len(f) < 3 || stopwords[f] || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// stopwords is a small English/Dutch set; campaign keywords rarely hinge on these.
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "you": true, "your": true,
	"are": true, "our": true, "from": true, "this": true, "that": true, "all": true,
	"een": true, "van": true, "het": true, "met": true, "voor": true, "del": true,
}

func metaContent(doc *goquery.Document, name string) (string, bool) {
	var content string
	found := false
	doc.Find("meta[name]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if a, ok := s.Attr("name"); ok && strings.EqualFold(a, name) {
			content, _ = s.Attr("content")
			found = true
			return false
		}
		return true
	})
	return strings.TrimSpace(content), found
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

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
