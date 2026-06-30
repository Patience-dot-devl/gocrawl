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

// TestAgent reports whether the given path is allowed for userAgent.
func (r *RobotsData) TestAgent(path, userAgent string) bool {
	if r == nil || r.data == nil {
		return true
	}
	return r.data.TestAgent(path, userAgent)
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
	Proxies         []*url.URL
	ProxyRotation   RotationStrategy
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
