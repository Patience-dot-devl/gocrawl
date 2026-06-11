// Package geo assesses Generative Engine Optimization: whether AI answer engines
// (ChatGPT, Perplexity, Google AI Overviews, Gemini, Claude) can access the site and trust
// its content enough to cite it.
//
// It combines three views of the crawl:
//   - per host, the robots.txt policy toward known AI crawler user-agents;
//   - per site, the presence of an /llms.txt content map (fetched on demand, like sitemap);
//   - per page, citability signals — author attribution, dates, and a main-content landmark
//     that lets extractors isolate the answer.
package geo

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/PuerkitoBio/goquery"
)

// aiCrawlers are the user-agents generative engines use to crawl and train on the web.
var aiCrawlers = []string{
	"GPTBot",             // OpenAI training
	"OAI-SearchBot",      // OpenAI search
	"ChatGPT-User",       // OpenAI on-demand fetch
	"ClaudeBot",          // Anthropic
	"anthropic-ai",       // Anthropic (legacy)
	"PerplexityBot",      // Perplexity
	"Google-Extended",    // Google Gemini / Vertex
	"Applebot-Extended",  // Apple Intelligence
	"CCBot",              // Common Crawl (feeds many LLMs)
	"Bytespider",         // ByteDance
	"meta-externalagent", // Meta AI
}

// articleTypes are the schema.org @types that mark editorial content where author and date
// attribution materially affect citability.
var articleTypes = map[string]bool{
	"Article": true, "NewsArticle": true, "BlogPosting": true, "TechArticle": true,
	"ScholarlyArticle": true, "Report": true, "Review": true,
}

// minProseWords gates the content-heavy checks (main landmark, JS-dependency, quotable
// density) so they only fire on pages with enough prose to judge.
const minProseWords = 300

// maxRawShareForJSDependent is the largest fraction of the rendered prose that the pre-JS
// HTML may contain before the page counts as JS-dependent. Below this, a non-executing AI
// crawler sees less than half the content the browser does.
const maxRawShareForJSDependent = 0.5

// minQuotablePer100Words is the floor for concrete, citable data points (numbers, percentages,
// currency) per 100 words of prose. Content below it reads as opinion or fluff with little for
// an answer engine to attribute, so it is a weaker citation candidate.
const minQuotablePer100Words = 0.5

// quotableRe matches concrete data points LLMs preferentially cite: currency amounts,
// percentages, and plain or grouped numbers (years, counts, measurements).
var quotableRe = regexp.MustCompile(`[$€£¥]\s?\d[\d,.]*|\d[\d,.]*\s?%|\b\d[\d,.]*\b`)

// Analyzer reports on AI-crawler access and content citability. It fetches /llms.txt with the
// given fetcher, regardless of the crawl's render mode.
type Analyzer struct {
	fetcher crawler.Fetcher
	// quotableDensity enables the opt-in quotable-data-density check (geo-low-quotable-density).
	// It is a lower-confidence heuristic, off by default; see Option.
	quotableDensity bool
}

// Option configures a GEO analyzer.
type Option func(*Analyzer)

// WithQuotableDensity enables the opt-in quotable-data-density check (geo-low-quotable-density),
// which is off by default.
func WithQuotableDensity(on bool) Option { return func(a *Analyzer) { a.quotableDensity = on } }

// New returns a GEO analyzer that fetches /llms.txt with the given fetcher, configured by opts.
func New(fetcher crawler.Fetcher, opts ...Option) *Analyzer {
	a := &Analyzer{fetcher: fetcher}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (Analyzer) Name() string { return "geo" }
func (Analyzer) Description() string {
	return "Generative Engine Optimization: AI-crawler robots.txt policy, /llms.txt presence, author/date/main-content citability, JS-dependent content, and quotable-data density"
}

func (a Analyzer) Analyze(ctx context.Context, result *crawler.Result) []analyze.Issue {
	var issues []analyze.Issue
	issues = append(issues, a.crawlerPolicy(result)...)
	issues = append(issues, a.llmsTxt(ctx, result)...)
	issues = append(issues, analyze.EachPage(result, a.analyzePage)...)
	return issues
}

// crawlerPolicy reports, per host with a parsed robots.txt, which AI crawlers are disallowed
// from the site root. Blocking them is a legitimate choice, so this is informational — the
// point is to surface a policy that is often set unintentionally.
func (a Analyzer) crawlerPolicy(result *crawler.Result) []analyze.Issue {
	var issues []analyze.Issue
	hosts := make([]string, 0, len(result.Robots))
	for host := range result.Robots {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	for _, host := range hosts {
		data := result.Robots[host]
		if data == nil || !data.Found {
			continue // no robots.txt means AI crawlers are allowed by default
		}
		var blocked []string
		for _, ua := range aiCrawlers {
			if !data.TestAgent("/", ua) {
				blocked = append(blocked, ua)
			}
		}
		if len(blocked) > 0 {
			issues = append(issues, analyze.Issue{
				Analyzer: "geo", URL: "host " + host, Severity: analyze.Info,
				Code: "geo-ai-crawler-blocked", Message: "robots.txt disallows AI crawlers at the site root",
				Data: map[string]any{"blocked": blocked},
			})
		}
	}
	return issues
}

// llmsTxt checks the seed host for an /llms.txt content map.
func (a Analyzer) llmsTxt(ctx context.Context, result *crawler.Result) []analyze.Issue {
	seed, err := url.Parse(result.Seed)
	if err != nil || seed.Host == "" {
		return nil
	}
	base := seed.Scheme + "://" + seed.Host
	llmsURL := base + "/llms.txt"

	page, ferr := a.fetcher.Fetch(ctx, llmsURL)
	if ferr == nil && page != nil && page.StatusCode == 200 && len(page.Body) > 0 {
		return []analyze.Issue{{
			Analyzer: "geo", URL: llmsURL, Severity: analyze.Info,
			Code: "geo-llms-txt", Message: "Site publishes an /llms.txt content map",
		}}
	}
	return []analyze.Issue{{
		Analyzer: "geo", URL: llmsURL, Severity: analyze.Info,
		Code: "geo-no-llms-txt", Message: "No /llms.txt content map found at the site root",
	}}
}

func (a Analyzer) analyzePage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	doc := p.Doc
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "geo", URL: p.FinalURL, Severity: sev, Code: code, Message: msg, Data: data})
	}

	objs := jsonldObjects(doc)

	// Editorial pages without clear author/date attribution are weaker citation candidates.
	if isArticle(doc, objs) {
		if !hasAuthor(doc, objs) {
			add(analyze.Info, "geo-missing-author", "Article-like page has no author attribution", nil)
		}
		if !hasDate(doc, objs) {
			add(analyze.Info, "geo-missing-date", "Article-like page has no published or modified date", nil)
		}
	}

	proseWords := countProseWords(doc)

	// A main-content landmark lets answer engines isolate the answer from chrome.
	if doc.Find("main, article").Length() == 0 && proseWords >= minProseWords {
		add(analyze.Info, "geo-no-main-landmark", "Content-heavy page has no <main> or <article> landmark",
			map[string]any{"words": proseWords})
	}

	// Content that only appears after JavaScript runs is invisible to most AI crawlers, which
	// fetch raw HTML without executing scripts. RawBody is populated only in headless mode.
	if proseWords >= minProseWords && len(p.RawBody) > 0 {
		if rawDoc, err := goquery.NewDocumentFromReader(bytes.NewReader(p.RawBody)); err == nil {
			rawWords := countProseWords(rawDoc)
			if float64(rawWords) < maxRawShareForJSDependent*float64(proseWords) {
				add(analyze.Info, "geo-js-dependent-content",
					"Most page content appears only after JavaScript runs; non-executing AI crawlers will miss it",
					map[string]any{"rendered_words": proseWords, "raw_words": rawWords})
			}
		}
	}

	// Concrete, attributable facts (numbers, stats, dates) are what answer engines quote.
	// Prose with almost none is a weaker citation candidate. Opt-in: this heuristic is off
	// unless the analyzer was built WithQuotableDensity.
	if a.quotableDensity && proseWords >= minProseWords {
		dataPoints := len(quotableRe.FindAllString(proseText(doc), -1))
		if float64(dataPoints) < minQuotablePer100Words*float64(proseWords)/100 {
			add(analyze.Info, "geo-low-quotable-density",
				"Content-heavy page has few concrete, citable data points (numbers, stats, dates)",
				map[string]any{"words": proseWords, "data_points": dataPoints})
		}
	}

	return issues
}

// proseText returns the page's main prose (paragraph and list text) collapsed to single spaces.
func proseText(doc *goquery.Document) string {
	return strings.Join(strings.Fields(doc.Find("p, li").Text()), " ")
}

// countProseWords counts words of paragraph and list prose, the same content basis used across
// the GEO content checks.
func countProseWords(doc *goquery.Document) int {
	return len(strings.Fields(doc.Find("p, li").Text()))
}

// isArticle reports whether the page presents as editorial content, via an <article> element
// or an article-type JSON-LD object.
func isArticle(doc *goquery.Document, objs []map[string]any) bool {
	if doc.Find("article").Length() > 0 {
		return true
	}
	for _, o := range objs {
		for _, t := range asStrings(o["@type"]) {
			if articleTypes[t] {
				return true
			}
		}
	}
	return false
}

func hasAuthor(doc *goquery.Document, objs []map[string]any) bool {
	if jsonldHasKey(objs, "author") {
		return true
	}
	if doc.Find(`[rel="author"], [itemprop="author"], meta[property="article:author"]`).Length() > 0 {
		return true
	}
	if c := metaName(doc, "author"); c != "" {
		return true
	}
	return false
}

func hasDate(doc *goquery.Document, objs []map[string]any) bool {
	if jsonldHasKey(objs, "datePublished") || jsonldHasKey(objs, "dateModified") {
		return true
	}
	return doc.Find(`time[datetime], meta[property="article:published_time"], meta[property="article:modified_time"]`).Length() > 0
}

// metaName returns the trimmed content of <meta name="..."> (case-insensitive), or "".
func metaName(doc *goquery.Document, name string) string {
	var content string
	doc.Find("meta[name]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if a, ok := s.Attr("name"); ok && strings.EqualFold(a, name) {
			content, _ = s.Attr("content")
			return false
		}
		return true
	})
	return strings.TrimSpace(content)
}

// jsonldObjects returns every object found in the page's JSON-LD, flattened across @graph
// arrays and top-level arrays.
func jsonldObjects(doc *goquery.Document) []map[string]any {
	var out []map[string]any
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}
		var v any
		if json.Unmarshal([]byte(raw), &v) != nil {
			return
		}
		out = append(out, flattenObjects(v)...)
	})
	return out
}

func flattenObjects(v any) []map[string]any {
	var out []map[string]any
	switch t := v.(type) {
	case map[string]any:
		out = append(out, t)
		if g, ok := t["@graph"]; ok {
			out = append(out, flattenObjects(g)...)
		}
	case []any:
		for _, item := range t {
			out = append(out, flattenObjects(item)...)
		}
	}
	return out
}

func jsonldHasKey(objs []map[string]any, key string) bool {
	for _, o := range objs {
		if v, ok := o[key]; ok && !isEmptyValue(v) {
			return true
		}
	}
	return false
}

func isEmptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

func asStrings(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
