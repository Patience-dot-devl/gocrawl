// Package crawler implements gocrawl's concurrent crawl engine. It fetches pages within
// a configurable scope, captures redirect chains and robots.txt data, and produces a
// Result that analyzers consume. The engine has no knowledge of specific SEO/SEA checks.
package crawler

import (
	"context"
	"net/http"
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
	URL      string `json:"url"`    // absolute, normalized
	Anchor   string `json:"anchor"` // visible text
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
	Implemented bool    `json:"implemented"`
	Note        string  `json:"note,omitempty"`
	LCP         float64 `json:"lcp_ms,omitempty"`
	FCP         float64 `json:"fcp_ms,omitempty"`
	CLS         float64 `json:"cls,omitempty"`
	TBT         float64 `json:"tbt_ms,omitempty"`
	TTFB        float64 `json:"ttfb_ms,omitempty"`
}

// Page is the unit passed from the engine to analyzers.
type Page struct {
	RequestedURL string            `json:"requested_url"`
	FinalURL     string            `json:"final_url"`
	StatusCode   int               `json:"status_code"`
	Header       http.Header       `json:"-"`
	ContentType  string            `json:"content_type"`
	Body         []byte            `json:"-"`
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

	index map[string]*Page // normalized URL -> page
}

// Page looks up a crawled page by URL (matched against requested and final URLs after
// normalization). The second return value reports whether it was found.
func (r *Result) Page(rawURL string) (*Page, bool) {
	if r.index == nil {
		return nil, false
	}
	p, ok := r.index[normalizeURL(rawURL)]
	return p, ok
}

// Options controls crawl scope and politeness.
type Options struct {
	MaxDepth        int
	MaxPages        int
	Concurrency     int
	RatePerSecond   float64
	UserAgent       string
	Include         []*regexp.Regexp
	Exclude         []*regexp.Regexp
	RespectRobots   bool
	AllowSubdomains bool
	FollowExternal  bool
	FollowNofollow  bool
	Timeout         time.Duration
	MaxBodyBytes    int64
	MaxRedirects    int
}

// DefaultOptions returns conservative, polite defaults.
func DefaultOptions() Options {
	return Options{
		MaxDepth:      2,
		MaxPages:      500,
		Concurrency:   4,
		RatePerSecond: 0,
		UserAgent:     "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)",
		RespectRobots: true,
		Timeout:       15 * time.Second,
		MaxBodyBytes:  5 << 20,
		MaxRedirects:  10,
	}
}
