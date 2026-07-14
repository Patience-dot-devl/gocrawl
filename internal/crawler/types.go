// Package crawler implements gocrawl's concurrent crawl engine. It fetches pages within
// a configurable scope, captures redirect chains and robots.txt data, and produces a
// Result that analyzers consume. The engine has no knowledge of specific SEO/SEA checks.
package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/temoto/robotstxt"
)

// Fetcher retrieves a single URL and returns a populated Page. Implementations include
// the raw HTTPFetcher and the (stubbed) headless renderer.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) (*Page, error)
}

// Redirect is one hop in a redirect chain.
type Redirect struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Status int    `json:"status"`
}

// Link is an outbound link discovered on a page.
type Link struct {
	URL      string `json:"url"`                // absolute, normalized for dedup (trailing slash stripped)
	Resolved string `json:"resolved,omitempty"` // absolute, normalized but trailing slash preserved
	Anchor   string `json:"anchor"`             // visible text
	Rel      string `json:"rel"`
	Nofollow bool   `json:"nofollow"`
	External bool   `json:"external"` // points outside the seed host
}

// RenderResult holds headless-rendering output, including lab-mode Core Web Vitals.
// Implemented is true when chromedp produced real measurements; false when rendering
// fell back to a raw fetch (Note carries the reason). Metric times are milliseconds;
// CLS is the unitless layout-shift score. Zero means not collected.
//
// INP is a field-only metric and is not collected in lab mode. TBT (Total Blocking
// Time) is reported as a lab-mode proxy for responsiveness, matching Lighthouse.
type RenderResult struct {
	Implemented bool   `json:"implemented"`
	Note        string `json:"note,omitempty"`
	// RawFallback is set when the rendered DOM came back substantially thinner than the raw
	// HTML — a sign the page had not finished rendering when it was snapshotted. The analyzed
	// Body/Doc are then taken from the raw fetch (so structural checks like the H1 and meta
	// tags aren't false-negatives), while the Core Web Vitals below stay from the render but
	// should be treated as unreliable for this page. RenderedBytes/RawBytes record the sizes
	// that triggered it.
	RawFallback   bool    `json:"raw_fallback,omitempty"`
	RenderedBytes int     `json:"rendered_bytes,omitempty"`
	RawBytes      int     `json:"raw_bytes,omitempty"`
	LCP           float64 `json:"lcp_ms,omitempty"`
	FCP           float64 `json:"fcp_ms,omitempty"`
	CLS           float64 `json:"cls,omitempty"`
	TBT           float64 `json:"tbt_ms,omitempty"`
	TTFB          float64 `json:"ttfb_ms,omitempty"`

	// DataLayerPresent reports whether window.dataLayer was an array after the page rendered.
	DataLayerPresent bool `json:"data_layer_present,omitempty"`
	// DataLayer is the post-render snapshot of window.dataLayer, each element the raw JSON of
	// one entry (a GTM event push or a gtag() arguments object). It is deliberately not
	// serialized into reports: it can carry PII, so the datalayer analyzer emits sanitized
	// findings from it instead. Nil in raw mode or when the page has no dataLayer.
	DataLayer []json.RawMessage `json:"-"`
	// Requests holds outbound request URLs observed during render, bounded to avoid unbounded
	// growth. The datalayer analyzer uses it to confirm analytics/marketing tags actually
	// fired a network beacon. Not serialized.
	Requests []string `json:"-"`
}

// Page is the unit passed from the engine to analyzers.
type Page struct {
	RequestedURL string            `json:"requested_url"`
	FinalURL     string            `json:"final_url"`
	StatusCode   int               `json:"status_code"`
	Header       http.Header       `json:"-"`
	ContentType  string            `json:"content_type"`
	Body         []byte            `json:"-"`
	RawBody      []byte            `json:"-"` // pre-JS HTML captured during headless render; nil in raw mode
	Doc          *goquery.Document `json:"-"` // nil if not HTML or parse failed
	Redirects    []Redirect        `json:"redirects,omitempty"`
	Links        []Link            `json:"-"`
	Depth        int               `json:"depth"`
	Referrer     string            `json:"referrer,omitempty"`
	Duration     time.Duration     `json:"duration_ms"`
	FetchedAt    time.Time         `json:"fetched_at"`
	Err          string            `json:"error,omitempty"`
	Render       *RenderResult     `json:"render,omitempty"`
	// Truncated reports whether Body was cut short of the real response — either because it
	// hit the fetcher's body-size cap or because the connection failed partway through the
	// read. A truncated body may be missing elements (e.g. </head>, the closing tag of a
	// sitemap) that a downstream analyzer would otherwise expect to find, so treat findings
	// like "missing title" or "invalid sitemap" on a truncated page with that in mind.
	Truncated bool `json:"truncated,omitempty"`
}

// IsHTML reports whether the page body was parsed as an HTML document.
func (p *Page) IsHTML() bool { return p.Doc != nil }

// RobotsData is the parsed robots.txt for a single host.
type RobotsData struct {
	Host     string   `json:"host"`
	Found    bool     `json:"found"`
	Status   int      `json:"status"`
	Sitemaps []string `json:"sitemaps,omitempty"`
	data     *robotstxt.RobotsData
}

// TestAgent reports whether the given path is allowed for userAgent. Per RFC 9309, a 4xx
// robots.txt response means no rules apply (allow all); an unreachable robots.txt (network
// error or 5xx) means we can't know the site's intent, so crawling is disallowed until it's
// reachable. A 200 response we simply couldn't parse degrades to allow-all.
func (r *RobotsData) TestAgent(path, userAgent string) bool {
	if r == nil {
		return true
	}
	if r.data != nil {
		return r.data.TestAgent(path, userAgent)
	}
	if r.Status == 0 || r.Status >= 500 {
		return false
	}
	return true
}

// Result is the complete output of a crawl, consumed by analyzers.
type Result struct {
	Seed      string                 `json:"seed"`
	Pages     []*Page                `json:"pages"`
	Robots    map[string]*RobotsData `json:"robots,omitempty"`
	StartedAt time.Time              `json:"started_at"`
	Finished  time.Time              `json:"finished_at"`
	Opts      Options                `json:"-"`

	// ThrottleEvents counts how many times adaptive delay reduced the crawl rate after HTTP
	// 429/503 responses. FinalRate is the requests-per-second in effect when the crawl ended;
	// it is meaningful only when ThrottleEvents > 0.
	ThrottleEvents int     `json:"throttle_events,omitempty"`
	FinalRate      float64 `json:"final_rate,omitempty"`

	// Coverage reports whether the crawl visited every in-scope URL it discovered, or stopped
	// at a configured limit. When not Complete, findings that depend on fetching a page —
	// broken links above all — may be incomplete.
	Coverage Coverage `json:"coverage"`

	index map[string]*Page // normalized URL -> page
}

// Coverage summarizes how much of the in-scope site the crawl actually fetched. It exists so
// the report can warn that "0 broken links" might mean "the broken ones weren't reached"
// rather than "the site is clean".
type Coverage struct {
	// Complete is true when no in-scope, robots-allowed URL was left un-fetched because of a
	// depth or page-count limit.
	Complete bool `json:"complete"`
	// DiscoveredNotCrawled is the number of distinct in-scope URLs that were discovered but
	// never fetched because a limit was reached.
	DiscoveredNotCrawled int `json:"discovered_not_crawled,omitempty"`
	// PageLimitReached / DepthLimitReached record which configured limit cut the crawl short.
	PageLimitReached  bool `json:"page_limit_reached,omitempty"`
	DepthLimitReached bool `json:"depth_limit_reached,omitempty"`
	// Interrupted is true when the crawl's context was canceled before it finished on its own
	// (e.g. an operator's Ctrl-C) rather than a configured limit being reached. The site may
	// be far less covered than DiscoveredNotCrawled reflects, since much of it may never have
	// been discovered at all.
	Interrupted bool `json:"interrupted,omitempty"`
	// DurationLimitReached is true when the crawl stopped because it hit its --max-duration
	// wall-clock budget rather than being interrupted externally. Also implies Interrupted,
	// since the same context-cancellation path stops the crawl either way.
	DurationLimitReached bool `json:"duration_limit_reached,omitempty"`
	// MaxPages / MaxDepth echo the limits in effect (0 = unlimited), for the report message.
	MaxPages int `json:"max_pages,omitempty"`
	MaxDepth int `json:"max_depth,omitempty"`
}

// Page looks up a crawled page by URL (matched against requested and final URLs after
// normalization). The second return value reports whether it was found.
func (r *Result) Page(rawURL string) (*Page, bool) {
	if r.index == nil {
		return nil, false
	}
	p, ok := r.index[normalizeURL(rawURL, r.Opts.StripQuery)]
	return p, ok
}

// ResolveHref resolves href relative to from's own URL (mirroring how the crawl engine
// resolves an <a href> against a page's base — including an in-document <base href>, if any —
// via extractLinks) and looks up the resulting page in the crawl result. It exists for
// analyzers that read a raw href straight from the DOM (e.g. <link rel="amphtml"|"next"|"prev">)
// rather than from the engine's own extracted Links, so a relative href is resolved the same
// way an anchor link would be instead of failing the lookup outright. resolved is the
// slash-preserving absolute form, suitable for LinkPointsToRedirect. ok is false for an
// unusable href (empty, fragment-only, non-http(s) after resolution) or one with no match in
// the crawl.
func (r *Result) ResolveHref(from *Page, href string) (target *Page, resolved string, ok bool) {
	if r.index == nil || from == nil {
		return nil, "", false
	}
	base, err := url.Parse(from.FinalURL)
	if err != nil {
		return nil, "", false
	}
	key, resolvedURL, ok := resolveURL(base, href, r.Opts.StripQuery)
	if !ok {
		return nil, "", false
	}
	p, found := r.index[key]
	return p, resolvedURL, found
}

// Reindex rebuilds the URL lookup index from r.Pages. The engine populates the index
// incrementally during a crawl, so production code never needs this; it lets callers that
// construct a Result by hand (notably tests) make Page lookups resolve.
func (r *Result) Reindex() {
	r.index = make(map[string]*Page, len(r.Pages)*2)
	for _, p := range r.Pages {
		if p == nil {
			continue
		}
		r.index[normalizeURL(p.RequestedURL, r.Opts.StripQuery)] = p
		if p.FinalURL != "" {
			r.index[normalizeURL(p.FinalURL, r.Opts.StripQuery)] = p
		}
	}
}

// Options controls crawl scope and politeness.
type Options struct {
	MaxDepth      int
	MaxPages      int
	Concurrency   int
	RatePerSecond float64
	UserAgent     string
	// UserAgents is an optional pool of User-Agent strings to rotate across. When non-empty it
	// supersedes UserAgent; UserAgentRotation picks one per request.
	UserAgents        []string
	UserAgentRotation RotationStrategy
	// Proxies is an optional pool of proxy URLs (http, https, or socks5) to route requests
	// through. When non-empty, ProxyRotation picks one per request. Empty leaves Go's default
	// proxy behavior (honoring HTTP_PROXY/HTTPS_PROXY/NO_PROXY) in place.
	Proxies       []*url.URL
	ProxyRotation RotationStrategy
	// BasicAuthUser and BasicAuthPass, when BasicAuthUser is non-empty, are sent as an HTTP
	// Basic Authorization header on every request. This targets server-level Basic Auth (the
	// realm challenge common on staging/acceptance environments), sent preemptively rather
	// than in response to a 401 challenge.
	BasicAuthUser   string
	BasicAuthPass   string
	Include         []*regexp.Regexp
	Exclude         []*regexp.Regexp
	RespectRobots   bool
	AllowSubdomains bool
	FollowExternal  bool
	FollowNofollow  bool
	// StripQuery drops the query string when normalizing URLs, so URLs that differ only by
	// their query collapse to one and are crawled once.
	StripQuery   bool
	Timeout      time.Duration
	MaxBodyBytes int64
	MaxRedirects int
	// Verbose logs each fetch and every rate-limit change to stderr while crawling.
	Verbose bool
	// AdaptiveDelay automatically slows the crawl (halving requests-per-second, honoring any
	// Retry-After header) when the server responds with HTTP 429 or 503.
	AdaptiveDelay bool
	// MaxDuration bounds the crawl's total wall-clock time. Zero means unlimited. When it
	// elapses, the crawl stops early and still returns everything fetched so far as a
	// partial result (see Coverage.DurationLimitReached), the same way a canceled context
	// (e.g. an operator's Ctrl-C) does.
	MaxDuration time.Duration
}

// DefaultOptions returns conservative, polite defaults. By default the crawl is bounded by
// the total page budget (MaxPages), not by link depth (MaxDepth = 0 = unlimited): a shallow
// depth cap silently hides whole sections of a site — and the broken links in them — whereas
// a page budget walks the site breadth-first and stops at a predictable, reported size.
func DefaultOptions() Options {
	return Options{
		MaxDepth:      0,
		MaxPages:      500,
		Concurrency:   4,
		RatePerSecond: 0,
		UserAgent:     "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)",
		RespectRobots: true,
		Timeout:       15 * time.Second,
		MaxBodyBytes:  5 << 20,
		MaxRedirects:  10,
		AdaptiveDelay: true,
	}
}
