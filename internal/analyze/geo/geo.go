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
	"context"
	"encoding/json"
	"net/url"
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

// minProseWords gates the main-landmark check so it only fires on content-heavy pages.
const minProseWords = 300

// Analyzer reports on AI-crawler access and content citability. It fetches /llms.txt with the
// given fetcher, regardless of the crawl's render mode.
type Analyzer struct {
	fetcher crawler.Fetcher
}

// New returns a GEO analyzer that fetches /llms.txt with the given fetcher.
func New(fetcher crawler.Fetcher) *Analyzer { return &Analyzer{fetcher: fetcher} }

func (Analyzer) Name() string { return "geo" }
func (Analyzer) Description() string {
	return "Generative Engine Optimization: AI-crawler robots.txt policy, /llms.txt presence, author/date/main-content citability signals"
}

func (a Analyzer) Analyze(ctx context.Context, result *crawler.Result) []analyze.Issue {
	var issues []analyze.Issue
	issues = append(issues, a.crawlerPolicy(result)...)
	issues = append(issues, a.llmsTxt(ctx, result)...)
	issues = append(issues, analyze.EachPage(result, analyzePage)...)
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

func analyzePage(p *crawler.Page) []analyze.Issue {
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

	// A main-content landmark lets answer engines isolate the answer from chrome.
	if doc.Find("main, article").Length() == 0 {
		if words := len(strings.Fields(doc.Find("p, li").Text())); words >= minProseWords {
			add(analyze.Info, "geo-no-main-landmark", "Content-heavy page has no <main> or <article> landmark",
				map[string]any{"words": words})
		}
	}

	return issues
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
