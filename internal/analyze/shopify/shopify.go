// Package shopify detects Shopify storefronts and runs Shopify-specific checks: which schema
// each page template should carry and does not, structured data injected by two competing
// sources, product markup that flattens away its variants, and the crawlable utility and
// faceted URLs Shopify generates by default.
//
// Like the wordpress analyzer it stays completely silent on a site it does not recognize, so
// enabling it costs nothing on the rest of the web.
package shopify

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Option configures the analyzer.
type Option func(*Analyzer)

// WithProbes enables the checks that fetch an extra resource (currently the /products.json
// feed). Off by default, and wired to the same --specialized flag as the WordPress security
// probes, because an audit should not make requests the crawl did not already make unless
// the operator asked for it.
func WithProbes(on bool) Option { return func(a *Analyzer) { a.probe = on } }

// Analyzer runs the Shopify-specific checks.
type Analyzer struct {
	fetcher crawler.Fetcher
	probe   bool
}

// New returns a Shopify analyzer. The fetcher is used only by the opt-in probes.
func New(fetcher crawler.Fetcher, opts ...Option) *Analyzer {
	a := &Analyzer{fetcher: fetcher}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (Analyzer) Name() string { return "shopify" }
func (Analyzer) Description() string {
	return "Shopify detection plus store-specific checks: per-template structured-data coverage, theme/app schema conflicts, flattened product variants, and crawlable utility and faceted URLs"
}

func (a Analyzer) Analyze(ctx context.Context, result *crawler.Result) []analyze.Issue {
	s := detect(result)
	if !s.detected {
		return nil
	}
	base := analyze.SiteBase(result)

	data := map[string]any{"signals": sortedKeys(s.signals)}
	if s.theme != "" {
		data["theme"] = s.theme
	}
	if s.themeID != "" {
		data["theme_id"] = s.themeID
	}
	issues := []analyze.Issue{{
		Analyzer: "shopify", URL: base, Severity: analyze.Info,
		Code: "shopify-detected", Message: "Site is a Shopify storefront", Data: data,
	}}
	issues = append(issues, templateGapIssues(result, base)...)
	return issues
}

// site is what detection learned about the store, aggregated across every crawled page.
type site struct {
	detected bool
	theme    string
	themeID  string
	signals  map[string]bool
}

// strongFingerprints and weakFingerprints are kept as two separate tiers, not one flat list,
// because they discriminate differently. strongFingerprints only appear in a Shopify theme's
// own layout (the inline bootstrap object and the storefront runtime's feature flag), so
// either one reliably means the crawled site IS a Shopify storefront. weakFingerprints are
// just Shopify CDN/asset URLs, and those also appear on a site that is NOT itself hosted on
// Shopify but embeds a Shopify Buy Button widget (a single product's checkout, dropped into a
// WordPress, Squarespace, or hand-rolled page): that page loads cdn.shopify.com assets and
// often references a *.myshopify.com backing store, with no other Shopify marker anywhere on
// the site. Live storefronts (kith.com, allbirds.com, redbullshopus.com) carry both tiers; a
// Buy Button host carries only the weak tier. Merging the tiers back into one list would make
// that Buy Button page indistinguishable from a real storefront, and detection drives Tasks
// 9-11's per-template schema checks, so a false positive there is not a stray info line but a
// wall of invented findings against templates that do not exist. A weak-only match therefore
// never sets s.detected — only a strong fingerprint or the X-ShopId/X-Shopify-Stage headers do
// (see detect below) — even though it costs us the rare Hydrogen (headless Shopify) storefront
// that strips both strong markers from its rendered output and so goes undetected. That is the
// trade-off this package makes throughout: like the wordpress analyzer, stay silent when in
// doubt, because one more silently-skipped site costs nothing but a confidently wrong report
// costs the reader's trust in the rest of it.
var (
	strongFingerprints = []string{
		"shopify.theme",
		"shopify-features",
	}
	weakFingerprints = []string{
		"cdn.shopify.com",
		".myshopify.com",
		"/cdn/shop/",
	}
)

// themeRe pulls the Shopify.theme object out of the inline bootstrap script the platform
// injects. The object is flat, so matching up to the first closing brace is enough.
var themeRe = regexp.MustCompile(`Shopify\.theme\s*=\s*(\{[^}]*\})`)

// detect scans every crawled HTML page for Shopify fingerprints and aggregates what it finds.
func detect(result *crawler.Result) site {
	s := site{signals: make(map[string]bool)}
	for _, p := range result.Pages {
		if p.Header.Get("X-ShopId") != "" || p.Header.Get("X-Shopify-Stage") != "" {
			s.detected = true
			s.signals["header"] = true
		}
		if !p.IsHTML() {
			continue
		}
		html := pageHTML(p)
		lower := strings.ToLower(html)
		for _, f := range strongFingerprints {
			if strings.Contains(lower, f) {
				s.detected = true
				s.signals[f] = true
			}
		}
		for _, f := range weakFingerprints {
			if strings.Contains(lower, f) {
				// Not sufficient on its own (see the doc comment above); record it as a
				// signal but do not flip s.detected.
				s.signals[f] = true
			}
		}
		if s.theme == "" {
			s.theme, s.themeID = parseTheme(html)
		}
	}
	return s
}

// parseTheme reads the theme name and id out of the Shopify.theme bootstrap object.
func parseTheme(html string) (name, id string) {
	m := themeRe.FindStringSubmatch(html)
	if m == nil {
		return "", ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(m[1]), &obj); err != nil {
		return "", ""
	}
	name, _ = obj["name"].(string)
	switch v := obj["id"].(type) {
	case string:
		id = v
	case float64:
		id = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return name, id
}

// pageHTML returns the page's HTML source. It prefers the raw body, falling back to
// re-serializing the parsed document, so fingerprints that live in inline scripts are visible
// either way.
func pageHTML(p *crawler.Page) string {
	if len(p.Body) > 0 {
		return string(p.Body)
	}
	if p.Doc == nil {
		return ""
	}
	html, err := p.Doc.Html()
	if err != nil {
		return ""
	}
	return html
}

// sortedKeys returns a set's members in a stable order.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
