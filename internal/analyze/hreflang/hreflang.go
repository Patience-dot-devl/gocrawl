// Package hreflang implements hreflang annotation validity, self-reference, x-default,
// and return-link reciprocity checks across the crawled page set.
package hreflang

import (
	"context"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/language"
)

// Analyzer validates hreflang annotations across pages.
type Analyzer struct{}

// New returns a new hreflang analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "hreflang" }
func (Analyzer) Description() string {
	return "hreflang annotation validity, self-reference, x-default, and return-link reciprocity"
}

// entry is a single hreflang annotation: a language code and its target href.
type entry struct {
	lang string
	href string
}

// isValidHreflang reports whether code is a well-formed BCP-47 language tag (which "x-default"
// itself parses as, being a valid private-use subtag). A hand-rolled regex here previously
// rejected legitimate real-world tags: UN M49 region codes (es-419), 4-letter script subtags
// (zh-Hant), lowercase regions (en-us), and 3-letter macrolanguage codes (fil).
func isValidHreflang(code string) bool {
	_, err := language.Parse(code)
	return err == nil
}

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	// First pass: collect hreflang clusters keyed by page FinalURL.
	clusters := map[string][]entry{}
	var pages []*crawler.Page
	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		pages = append(pages, p)
		clusters[p.FinalURL] = collect(p)
	}

	// Second pass: validate each page's cluster.
	var issues []analyze.Issue
	for _, p := range pages {
		cluster := clusters[p.FinalURL]
		if len(cluster) == 0 {
			continue
		}
		url := p.FinalURL
		add := func(sev analyze.Severity, code, msg string, data map[string]any) {
			issues = append(issues, analyze.Issue{Analyzer: "hreflang", URL: url, Severity: sev, Code: code, Message: msg, Data: data})
		}

		hasXDefault := false
		hasSelf := false
		for _, e := range cluster {
			if !isValidHreflang(e.lang) {
				add(analyze.Warning, "hreflang-invalid-code", "hreflang code is not a valid language tag", map[string]any{"code": e.lang})
			}
			if strings.EqualFold(e.lang, "x-default") {
				hasXDefault = true
			}
			if target, ok := result.Page(e.href); ok && target == p {
				hasSelf = true
			}
		}

		if !hasXDefault {
			add(analyze.Info, "hreflang-missing-x-default", "hreflang cluster has no x-default entry", nil)
		}
		if !hasSelf {
			add(analyze.Info, "hreflang-missing-self", "hreflang cluster has no self-referencing entry", nil)
		}

		// Return-link reciprocity: each target must point back to this page.
		for _, e := range cluster {
			target, ok := result.Page(e.href)
			if !ok || target == p {
				continue
			}
			if !pointsBack(clusters[target.FinalURL], result, p) {
				add(analyze.Warning, "hreflang-no-return-link", "hreflang target does not link back to this page", map[string]any{"target": e.href})
			}
		}
	}
	return issues
}

// collect reads the hreflang annotations from a page's <link rel="alternate" hreflang>.
func collect(p *crawler.Page) []entry {
	var out []entry
	p.Doc.Find(`link[rel="alternate"][hreflang]`).Each(func(_ int, s *goquery.Selection) {
		lang, _ := s.Attr("hreflang")
		href, _ := s.Attr("href")
		lang = strings.TrimSpace(lang)
		href = strings.TrimSpace(href)
		if lang == "" || href == "" {
			return
		}
		out = append(out, entry{lang: lang, href: href})
	})
	return out
}

// pointsBack reports whether any href in cluster resolves to the page back.
func pointsBack(cluster []entry, result *crawler.Result, back *crawler.Page) bool {
	for _, e := range cluster {
		if target, ok := result.Page(e.href); ok && target == back {
			return true
		}
	}
	return false
}
