// Package wordpress fingerprints WordPress sites and runs WordPress-specific checks that the
// generic analyzers do not name. It works in two passes: a passive pass over the crawled HTML
// (detection, version disclosure, front-end bloat, the default tagline, ugly permalinks) and an
// opt-in active pass that probes well-known endpoints for security and hygiene problems
// (xmlrpc.php, REST/author user enumeration, directory listing, readme.html).
//
// Most WordPress fingerprints live in the shared header/footer template, so they repeat on every
// page. To avoid flooding the report, the passive checks are aggregated across the crawl and
// emitted once per site (keyed by the site base URL); only ugly permalinks, which are genuinely
// per-page, fire per page.
//
// The active probes fetch a handful of extra URLs beyond the crawl, like the geo and sitemap
// analyzers. They are off by default and enabled via WithSecurityProbes (wired to the
// `specialized` config flag) so the default analyzer stays passive.
package wordpress

import (
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

// manyPluginsThreshold is the number of distinct plugins shipping front-end assets above which
// the page-weight pile-up is worth flagging. WordPress sites with this many plugins enqueuing
// CSS/JS commonly suffer render-blocking and HTTP-request bloat.
const manyPluginsThreshold = 10

// pluginRe extracts a plugin slug from a /wp-content/plugins/<slug>/... asset URL.
var pluginRe = regexp.MustCompile(`/wp-content/plugins/([a-zA-Z0-9._-]+)`)

// readmeVersionRe pulls the WordPress version out of readme.html ("Version 6.4.2").
var readmeVersionRe = regexp.MustCompile(`(?i)version\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)

// authorArchiveRe matches a WordPress author archive path (/author/<nicename>/, plus its
// pagination). dateArchiveRe matches a year / year-month / year-month-day archive and nothing
// else, so it never fires on a post served under a date-based permalink (which has a slug after
// the date segments).
var (
	authorArchiveRe = regexp.MustCompile(`^/author/[^/]+/?`)
	dateArchiveRe   = regexp.MustCompile(`^/\d{4}(/\d{2}(/\d{2})?)?/?$`)
)

// seoPlugins maps a lowercased body signal to the human name of the SEO plugin it indicates.
// Checked in order; the first match wins.
var seoPlugins = []struct{ signal, name string }{
	{"yoast", "Yoast SEO"},
	{"rank-math", "Rank Math"},
	{"rank math", "Rank Math"},
	{"aioseo", "All in One SEO"},
	{"all in one seo", "All in One SEO"},
}

// Analyzer fingerprints WordPress and runs WordPress-specific checks. The fetcher is used by the
// opt-in security probes; it is not touched when probes are disabled.
type Analyzer struct {
	fetcher crawler.Fetcher
	// probe enables the active security/hygiene endpoint probes (off by default; see Option).
	probe bool
}

// Option configures a WordPress analyzer.
type Option func(*Analyzer)

// WithSecurityProbes enables the active endpoint probes (xmlrpc.php, REST/author user
// enumeration, uploads directory listing, readme.html), which are off by default.
func WithSecurityProbes(on bool) Option { return func(a *Analyzer) { a.probe = on } }

// New returns a WordPress analyzer. The fetcher is used only by the opt-in security probes.
func New(fetcher crawler.Fetcher, opts ...Option) *Analyzer {
	a := &Analyzer{fetcher: fetcher}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (Analyzer) Name() string { return "wordpress" }
func (Analyzer) Description() string {
	return "WordPress detection plus WP-specific checks: version disclosure, plugin/emoji/jQuery-Migrate bloat, default tagline, ugly permalinks, conflicting SEO plugins, indexable attachment/search/archive pages, and (opt-in) xmlrpc/user-enumeration/directory-listing/readme probes"
}

// site holds what the passive pass learned about the WordPress install, aggregated across pages.
type site struct {
	detected       bool
	version        string          // WordPress core version, if disclosed via the generator tag
	seoPlugins     map[string]bool // detected SEO plugin names (more than one means a conflict)
	plugins        map[string]bool // distinct plugin slugs shipping front-end assets
	emoji          bool            // wp-emoji script loaded
	jqueryMigrate  bool            // jQuery Migrate shim loaded
	defaultTagline bool            // "Just another WordPress site"
}

func (a Analyzer) Analyze(ctx context.Context, result *crawler.Result) []analyze.Issue {
	s := detect(result)
	if !s.detected {
		return nil // not WordPress: stay silent
	}
	base := siteBase(result)

	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "wordpress", URL: base, Severity: sev, Code: code, Message: msg, Data: data})
	}

	plugins := sortedKeys(s.plugins)
	seoPlugins := sortedKeys(s.seoPlugins)
	detData := map[string]any{"plugins": plugins, "plugin_count": len(plugins)}
	if s.version != "" {
		detData["version"] = s.version
	}
	if len(seoPlugins) > 0 {
		detData["seo_plugin"] = seoPlugins[0]
	}
	add(analyze.Info, "wp-detected", "Site is built on WordPress", detData)

	// Version disclosure maps the install to known CVEs for that exact release.
	if s.version != "" {
		add(analyze.Warning, "wp-version-exposed", "WordPress version is disclosed in the generator meta tag",
			map[string]any{"version": s.version})
	}
	// Front-end bloat: each of these is commonly removable and adds requests/bytes sitewide.
	if s.emoji {
		add(analyze.Info, "wp-emoji-enabled", "wp-emoji script is loaded sitewide (usually safe to dequeue)", nil)
	}
	if s.jqueryMigrate {
		add(analyze.Info, "wp-jquery-migrate", "jQuery Migrate compatibility shim is loaded (remove if no legacy code needs it)", nil)
	}
	if len(plugins) >= manyPluginsThreshold {
		add(analyze.Warning, "wp-many-plugin-assets", "Many plugins ship front-end assets, risking render-blocking request pile-up",
			map[string]any{"plugins": plugins, "plugin_count": len(plugins)})
	}
	if s.defaultTagline {
		add(analyze.Warning, "wp-default-tagline", `Site uses the default "Just another WordPress site" tagline`, nil)
	}
	switch {
	case len(seoPlugins) == 0:
		add(analyze.Info, "wp-no-seo-plugin", "No SEO plugin (Yoast, Rank Math, All in One SEO) detected", nil)
	case len(seoPlugins) > 1:
		add(analyze.Warning, "wp-multiple-seo-plugins", "More than one SEO plugin is active, which produces conflicting/duplicate meta output",
			map[string]any{"plugins": seoPlugins})
	}

	// Per-page checks: ugly permalinks and indexable low-value pages vary URL by URL.
	issues = append(issues, analyze.EachPage(result, perPage)...)

	if a.probe && base != "" {
		issues = append(issues, a.securityProbes(ctx, base)...)
	}
	return issues
}

// detect scans every crawled HTML page for WordPress fingerprints and aggregates what it finds.
func detect(result *crawler.Result) site {
	s := site{plugins: map[string]bool{}, seoPlugins: map[string]bool{}}
	for _, p := range result.Pages {
		if !p.IsHTML() || p.StatusCode != 200 {
			continue
		}
		assets := assetURLs(p.Doc)
		body := strings.ToLower(string(p.Body))

		if gen := generator(p.Doc); strings.HasPrefix(strings.ToLower(gen), "wordpress") {
			s.detected = true
			if v := strings.TrimSpace(gen[len("WordPress"):]); v != "" && s.version == "" {
				s.version = v
			}
		}
		if headerSignals(p) || strings.Contains(assets, "/wp-content/") || strings.Contains(assets, "/wp-includes/") || strings.Contains(body, "/wp-json/") {
			s.detected = true
		}

		for _, m := range pluginRe.FindAllStringSubmatch(assets, -1) {
			s.plugins[m[1]] = true
		}
		if strings.Contains(assets, "wp-emoji-release") || strings.Contains(body, "_wpemojisettings") {
			s.emoji = true
		}
		if strings.Contains(assets, "jquery-migrate") {
			s.jqueryMigrate = true
		}
		if strings.Contains(body, "just another wordpress site") {
			s.defaultTagline = true
		}
		for _, sp := range seoPlugins {
			if strings.Contains(body, sp.signal) {
				s.seoPlugins[sp.name] = true
			}
		}
	}
	return s
}

// perPage runs the URL-dependent checks against a single page: the ugly-permalink check, and the
// indexable-low-value-page checks for WordPress's auto-generated thin/duplicate pages (attachment
// pages, internal search results, author and date archives). The low-value checks fire only when
// the page is actually indexable — a noindex tag or a canonical pointing elsewhere already
// handles it, so there is nothing to flag.
func perPage(p *crawler.Page) []analyze.Issue {
	if !p.IsHTML() || p.StatusCode != 200 {
		return nil
	}
	u, err := url.Parse(p.FinalURL)
	if err != nil {
		return nil
	}
	var issues []analyze.Issue
	add := func(sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "wordpress", URL: p.FinalURL, Severity: sev, Code: code, Message: msg, Data: data})
	}
	q := u.Query()

	for _, key := range []string{"p", "page_id", "cat"} {
		if q.Get(key) != "" {
			add(analyze.Info, "wp-ugly-permalink", "Page uses a default plain permalink (e.g. ?p=N) rather than a pretty URL",
				map[string]any{"param": key})
			break
		}
	}

	if indexable(p) {
		switch {
		case q.Get("attachment_id") != "":
			add(analyze.Warning, "wp-indexable-attachment", "Attachment page is indexable (thin auto-generated page; noindex or redirect it to the parent)",
				map[string]any{"id": q.Get("attachment_id")})
		case q.Get("s") != "":
			add(analyze.Warning, "wp-indexable-search", "Internal search results page is indexable (it should be noindex)", nil)
		case authorArchiveRe.MatchString(u.Path):
			add(analyze.Info, "wp-indexable-author-archive", "Author archive is indexable (often duplicates the blog index on single-author sites)", nil)
		case dateArchiveRe.MatchString(u.Path):
			add(analyze.Info, "wp-indexable-date-archive", "Date archive is indexable (thin, duplicate-prone listing)", nil)
		}
	}
	return issues
}

// indexable reports whether a 200 HTML page is open to indexing: no noindex directive (meta or
// header) and no canonical pointing to a different URL.
func indexable(p *crawler.Page) bool {
	if strings.Contains(strings.ToLower(metaNamed(p.Doc, "robots")), "noindex") {
		return false
	}
	if p.Header != nil && strings.Contains(strings.ToLower(p.Header.Get("X-Robots-Tag")), "noindex") {
		return false
	}
	return !canonicalElsewhere(p)
}

// canonicalElsewhere reports whether the page's <link rel="canonical"> resolves to a URL other
// than the page's own (host, path ignoring a trailing slash, and query), meaning it defers
// indexing to a different URL.
func canonicalElsewhere(p *crawler.Page) bool {
	href, ok := p.Doc.Find(`link[rel="canonical"]`).First().Attr("href")
	if !ok || strings.TrimSpace(href) == "" {
		return false
	}
	base, err := url.Parse(p.FinalURL)
	if err != nil {
		return false
	}
	ref, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return false
	}
	canon := base.ResolveReference(ref)
	return !strings.EqualFold(canon.Host, base.Host) ||
		strings.TrimRight(canon.Path, "/") != strings.TrimRight(base.Path, "/") ||
		canon.RawQuery != base.RawQuery
}

// securityProbes fetches well-known WordPress endpoints and reports exposure. Each probe is
// independent and best-effort; a failed fetch yields no finding.
func (a Analyzer) securityProbes(ctx context.Context, base string) []analyze.Issue {
	var issues []analyze.Issue
	add := func(u string, sev analyze.Severity, code, msg string, data map[string]any) {
		issues = append(issues, analyze.Issue{Analyzer: "wordpress", URL: u, Severity: sev, Code: code, Message: msg, Data: data})
	}

	// xmlrpc.php is a brute-force amplification and pingback-DDoS vector. A live endpoint answers
	// GET with 405 and a tell-tale body, or 200.
	if u := base + "/xmlrpc.php"; true {
		if page, err := a.fetcher.Fetch(ctx, u); err == nil && page != nil {
			body := strings.ToLower(string(page.Body))
			if (page.StatusCode == 200 || page.StatusCode == 405) && strings.Contains(body, "xml-rpc") {
				add(u, analyze.Warning, "wp-xmlrpc-enabled", "xmlrpc.php is enabled (brute-force amplification and pingback-DDoS vector)", nil)
			}
		}
	}

	// The REST users endpoint enumerates valid usernames unless filtered.
	if u := base + "/wp-json/wp/v2/users"; true {
		if page, err := a.fetcher.Fetch(ctx, u); err == nil && page != nil && page.StatusCode == 200 {
			if names := restUserSlugs(page.Body); len(names) > 0 {
				add(u, analyze.Warning, "wp-user-enumeration-rest", "REST API exposes usernames via /wp-json/wp/v2/users",
					map[string]any{"usernames": names, "count": len(names)})
			}
		}
	}

	// /?author=1 redirects to /author/<login>/ on installs that leak usernames this way.
	if u := base + "/?author=1"; true {
		if page, err := a.fetcher.Fetch(ctx, u); err == nil && page != nil {
			if login := authorFromURL(page.FinalURL); login != "" {
				add(u, analyze.Warning, "wp-user-enumeration-author", "Author archive redirect leaks a username (/?author=1 → /author/<login>/)",
					map[string]any{"username": login})
			}
		}
	}

	// A browsable uploads directory leaks file listings.
	if u := base + "/wp-content/uploads/"; true {
		if page, err := a.fetcher.Fetch(ctx, u); err == nil && page != nil && page.StatusCode == 200 {
			if strings.Contains(strings.ToLower(string(page.Body)), "index of") {
				add(u, analyze.Warning, "wp-directory-listing", "Uploads directory is browsable (directory listing enabled)", nil)
			}
		}
	}

	// readme.html ships the exact core version and should be removed in production.
	if u := base + "/readme.html"; true {
		if page, err := a.fetcher.Fetch(ctx, u); err == nil && page != nil && page.StatusCode == 200 {
			body := string(page.Body)
			if strings.Contains(strings.ToLower(body), "wordpress") {
				data := map[string]any{}
				if m := readmeVersionRe.FindStringSubmatch(body); m != nil {
					data["version"] = m[1]
				}
				add(u, analyze.Warning, "wp-readme-exposed", "readme.html is reachable and discloses the WordPress version", data)
			}
		}
	}
	return issues
}

// restUserSlugs parses a /wp-json/wp/v2/users response and returns the slugs (login names).
func restUserSlugs(body []byte) []string {
	var users []map[string]any
	if json.Unmarshal(body, &users) != nil {
		return nil
	}
	var out []string
	for _, u := range users {
		if slug, ok := u["slug"].(string); ok && slug != "" {
			out = append(out, slug)
		}
	}
	return out
}

// authorFromURL returns the login from an /author/<login>/ URL, or "".
func authorFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	const marker = "/author/"
	i := strings.Index(u.Path, marker)
	if i < 0 {
		return ""
	}
	login := strings.Trim(u.Path[i+len(marker):], "/")
	if j := strings.IndexByte(login, '/'); j >= 0 {
		login = login[:j]
	}
	return login
}

// assetURLs concatenates the src/href attributes of scripts, stylesheets, and images, where
// WordPress core, plugin, and theme paths surface.
func assetURLs(doc *goquery.Document) string {
	var parts []string
	doc.Find("script[src], img[src]").Each(func(_ int, s *goquery.Selection) {
		if v, ok := s.Attr("src"); ok {
			parts = append(parts, v)
		}
	})
	doc.Find("link[href]").Each(func(_ int, s *goquery.Selection) {
		if v, ok := s.Attr("href"); ok {
			parts = append(parts, v)
		}
	})
	return strings.Join(parts, "\n")
}

// generator returns the content of <meta name="generator">, or "".
func generator(doc *goquery.Document) string { return metaNamed(doc, "generator") }

// metaNamed returns the trimmed content of the first <meta name="..."> matching name
// (case-insensitive), or "".
func metaNamed(doc *goquery.Document, name string) string {
	var content string
	doc.Find("meta[name]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if n, ok := s.Attr("name"); ok && strings.EqualFold(n, name) {
			content, _ = s.Attr("content")
			return false
		}
		return true
	})
	return strings.TrimSpace(content)
}

// headerSignals reports WordPress-specific response headers: X-Pingback and the REST API
// discovery Link (rel="https://api.w.org/").
func headerSignals(p *crawler.Page) bool {
	if p.Header == nil {
		return false
	}
	if p.Header.Get("X-Pingback") != "" {
		return true
	}
	return strings.Contains(p.Header.Get("Link"), "api.w.org")
}

// siteBase returns the scheme://host of the crawl, derived from the seed or, failing that, the
// first crawled page's final URL.
func siteBase(result *crawler.Result) string {
	if u, err := url.Parse(result.Seed); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	for _, p := range result.Pages {
		if u, err := url.Parse(p.FinalURL); err == nil && u.Host != "" {
			return u.Scheme + "://" + u.Host
		}
	}
	return ""
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
