// Package analyze defines the analyzer pipeline: the Issue/Severity types, the Analyzer
// interface every check implements, and a Registry to hold them. Analyzers consume a
// crawler.Result and emit Issues; they never fetch the crawl themselves. This single seam
// is how new SEO/SEA checks are added without touching the engine.
package analyze

import (
	"context"
	"net/url"

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
