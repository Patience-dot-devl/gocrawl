// Package analyze defines the analyzer pipeline: the Issue/Severity types, the Analyzer
// interface every check implements, and a Registry to hold them. Analyzers consume a
// crawler.Result and emit Issues; they never fetch the crawl themselves. This single seam
// is how new SEO/SEA checks are added without touching the engine.
package analyze

import (
	"context"
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Severity classifies how important an Issue is.
type Severity string

const (
	Info    Severity = "info"
	Warning Severity = "warning"
	Error   Severity = "error"
)

// Issue is a single finding emitted by an analyzer.
type Issue struct {
	Analyzer string         `json:"analyzer"`
	URL      string         `json:"url"`
	Severity Severity       `json:"severity"`
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Data     map[string]any `json:"data,omitempty"`
}

// InstanceKey is the Data key an analyzer sets when it emits several findings that share one
// (Analyzer, Code, URL) — a site-wide rollup raised once per schema.org type, say. Its string
// value joins the finding's identity when two crawls are compared, so those findings stay
// distinct instead of collapsing into one. Findings without it keep the three-part identity,
// which is why adding it to an existing code re-keys that code against saved reports.
const InstanceKey = "instance"

// Analyzer is a single check. Implementations must be safe for sequential reuse.
type Analyzer interface {
	Name() string
	Description() string
	Analyze(ctx context.Context, result *crawler.Result) []Issue
}

// EachPage is a helper for per-page analyzers: it runs fn against every crawled page and
// concatenates the resulting issues.
func EachPage(result *crawler.Result, fn func(p *crawler.Page) []Issue) []Issue {
	var issues []Issue
	for _, p := range result.Pages {
		issues = append(issues, fn(p)...)
	}
	return issues
}

// Registry holds analyzers in registration order.
type Registry struct {
	byName map[string]Analyzer
	order  []string
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]Analyzer)}
}

// Register adds an analyzer. A later registration with the same name replaces the earlier.
func (r *Registry) Register(a Analyzer) {
	if _, exists := r.byName[a.Name()]; !exists {
		r.order = append(r.order, a.Name())
	}
	r.byName[a.Name()] = a
}

// All returns every analyzer in registration order.
func (r *Registry) All() []Analyzer {
	out := make([]Analyzer, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.byName[name])
	}
	return out
}

// Get returns the analyzer with the given name.
func (r *Registry) Get(name string) (Analyzer, bool) {
	a, ok := r.byName[name]
	return a, ok
}

// Names returns analyzer names in registration order.
func (r *Registry) Names() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Select returns the analyzers to run given enabled/disabled allow/deny lists. If enabled
// is non-empty, only those run (in registration order); otherwise all run except any in
// disabled. Unknown names in enabled are ignored.
func (r *Registry) Select(enabled, disabled []string) []Analyzer {
	deny := make(map[string]bool, len(disabled))
	for _, d := range disabled {
		deny[d] = true
	}
	if len(enabled) > 0 {
		allow := make(map[string]bool, len(enabled))
		for _, e := range enabled {
			allow[e] = true
		}
		var out []Analyzer
		for _, name := range r.order {
			if allow[name] && !deny[name] {
				out = append(out, r.byName[name])
			}
		}
		return out
	}
	var out []Analyzer
	for _, name := range r.order {
		if !deny[name] {
			out = append(out, r.byName[name])
		}
	}
	return out
}

// Run executes the given analyzers over result and concatenates their issues.
func Run(ctx context.Context, analyzers []Analyzer, result *crawler.Result) []Issue {
	var issues []Issue
	for _, a := range analyzers {
		issues = append(issues, a.Analyze(ctx, result)...)
	}
	return issues
}

// SiteBase returns the scheme://host of the crawl, derived from the seed or, failing that,
// from the first crawled page's final URL. Analyzers that aggregate a finding across the
// whole crawl — a server configuration, a CMS fingerprint, a template-wide schema gap — use
// it as the issue URL, so one site-wide fact does not repeat on every page of the report.
func SiteBase(result *crawler.Result) string {
	if u, err := url.Parse(result.Seed); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	for _, p := range result.Pages {
		if u, err := url.Parse(p.FinalURL); err == nil && u.Host != "" {
			return u.Scheme + "://" + u.Host
		}
	}
	return result.Seed
}

// DeclaredCanonical returns the page's <head> link[rel="canonical"] href resolved against the
// page's final URL, or "" when the page declares none or the href does not parse. Resolution
// matters: a theme emitting a relative href="/products/tee" would otherwise never equal the
// absolute URL it is compared with. Only <head> is searched, matching the seo analyzer, because
// search engines ignore a canonical in <body>. Callers that need to tell "no canonical" apart
// from "self-canonical" use this; callers that only need the page's identity use CanonicalURL.
func DeclaredCanonical(p *crawler.Page) string {
	if p.Doc == nil {
		return ""
	}
	href, _ := p.Doc.Find(`head link[rel="canonical"]`).First().Attr("href")
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	base, err := url.Parse(p.FinalURL)
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}

// CanonicalURL returns the URL a page declares as its canonical, resolved against its final
// URL, or the final URL itself when it declares none or the href does not parse. Site-wide
// counts key on it so that one product reached at /products/<h> and again at
// /collections/<c>/products/<h> counts as the single page it is. Compare two results through
// URLKey, not ==.
func CanonicalURL(p *crawler.Page) string {
	if c := DeclaredCanonical(p); c != "" {
		return c
	}
	return p.FinalURL
}

// URLKey normalizes a URL for identity comparison by dropping its fragment and any trailing
// slash, which differ without addressing a different page.
func URLKey(raw string) string {
	if i := strings.IndexByte(raw, '#'); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimRight(raw, "/")
}
