package shopify

import (
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/analyze/schemaorg"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// themeSource is the attribution given to a JSON-LD block with no recognizable app before it.
// A Shopify theme renders its structured data inline from Liquid, so it leaves no marker of
// its own — "not an app" is the only signal available, and it is the right default.
const themeSource = "theme"

// schemaApps maps a marker found in a script's URL or attributes to the app that owns it.
// Attribution reads attributes only, never a script's contents: an app's name routinely
// appears inside unrelated inline JSON, and matching on that would misattribute blocks.
var schemaApps = []struct{ marker, name string }{
	{"json-ld-for-seo", "JSON-LD for SEO"},
	{"jsonld-for-seo", "JSON-LD for SEO"},
	{"schemaplus", "Schema Plus"},
	{"schema-plus", "Schema Plus"},
	{"searchpie", "SearchPie"},
	{"schemaapp", "Schema App"},
	{"seoant", "SEOAnt"},
	{"yoast", "Yoast for Shopify"},
	{"tinyimg", "TinyIMG"},
	{"smart-seo", "Smart SEO"},
	{"avada-seo", "Avada SEO"},
}

// appFor returns the app owning a marker string, or "" when none matches.
func appFor(s string) string {
	lower := strings.ToLower(s)
	for _, app := range schemaApps {
		if strings.Contains(lower, app.marker) {
			return app.name
		}
	}
	return ""
}

// scriptMarkers returns the attribute text worth fingerprinting on a script element.
func scriptMarkers(s *goquery.Selection) string {
	src, _ := s.Attr("src")
	id, _ := s.Attr("id")
	class, _ := s.Attr("class")
	return src + " " + id + " " + class
}

// blockSources attributes each JSON-LD block on the page to the app that emitted it, indexed
// the same way schemaorg.Node.Block is: by position among the page's ld+json scripts in
// document order. A block is attributed by its own attributes first, then by the nearest
// recognized app script preceding it, and falls back to the theme.
func blockSources(doc *goquery.Document) []string {
	var sources []string
	current := themeSource
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		typ, _ := s.Attr("type")
		marker := scriptMarkers(s)
		if typ != "application/ld+json" {
			if app := appFor(marker); app != "" {
				current = app
			}
			return
		}
		if own := appFor(marker); own != "" {
			sources = append(sources, own)
			return
		}
		sources = append(sources, current)
	})
	return sources
}

// pageApps returns every schema app whose script appears on the page.
func pageApps(doc *goquery.Document) []string {
	seen := make(map[string]bool)
	var out []string
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		app := appFor(scriptMarkers(s))
		if app == "" || seen[app] {
			return
		}
		seen[app] = true
		out = append(out, app)
	})
	return out
}

// schemaIssues reports structured data emitted by two competing sources, and pages where an
// SEO app is installed but the raw HTML carries no JSON-LD at all.
func schemaIssues(p *crawler.Page, g schemaorg.Graph, tmpl Template) []analyze.Issue {
	var issues []analyze.Issue
	sources := blockSources(p.Doc)

	// Attribute each block that declares a page-level Product, then look for disagreement
	// about who owns it. Two blocks from one source are a duplicate, which the structured
	// analyzer already reports; two *sources* is the Shopify-specific failure, because
	// neither the theme nor the app knows the other exists.
	seen := make(map[string]bool)
	var attributed []string
	for _, n := range g.OfType("Product") {
		if n.Block >= len(sources) {
			continue
		}
		src := sources[n.Block]
		if seen[src] {
			continue
		}
		seen[src] = true
		attributed = append(attributed, src)
	}
	if len(attributed) > 1 {
		sort.Strings(attributed)
		issues = append(issues, analyze.Issue{
			Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Error,
			Code:    "shopify-schema-app-conflict",
			Message: "Product structured data is emitted by more than one source on this page",
			Data:    map[string]any{"sources": attributed},
		})
	}

	// A store that pays for an SEO app and shows no JSON-LD in the raw HTML is almost
	// certainly having it injected client-side, which a raw crawl cannot see. Say so rather
	// than reporting the page as bare.
	if len(g.Nodes) == 0 && tmpl != TemplateUtility && tmpl != TemplateUnknown {
		if apps := pageApps(p.Doc); len(apps) > 0 {
			issues = append(issues, analyze.Issue{
				Analyzer: "shopify", URL: p.FinalURL, Severity: analyze.Info,
				Code:    "shopify-schema-client-injected",
				Message: "A structured-data app is installed but the raw HTML carries no JSON-LD",
				Data:    map[string]any{"app": apps[0], "apps": apps, "template": string(tmpl)},
			})
		}
	}
	return issues
}
