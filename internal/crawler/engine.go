package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/time/rate"
)

// Adaptive-delay tuning. When the server signals overload (HTTP 429/503) the engine halves
// its requests-per-second down to backoffMinRate, ignoring repeat triggers that arrive within
// backoffDebounce of the last adjustment (a single burst of 429s should back off once, not
// once per concurrent worker).
const (
	backoffStartRate = 1.0             // req/s to drop to when the crawl was previously unrestricted
	backoffMinRate   = 0.1             // floor: one request every 10s
	backoffDebounce  = 2 * time.Second // ignore repeat triggers within this window
)

// Engine crawls a website concurrently within the configured scope.
type Engine struct {
	opts     Options
	fetcher  Fetcher
	robots   *robotsManager
	limiter  *rate.Limiter
	seedHost string

	// Adaptive-delay state, guarded by backoffMu.
	backoffMu     sync.Mutex
	baseRate      float64 // configured req/s (0 = unlimited)
	curRate       float64 // current req/s after any backoff (0 = still unlimited)
	lastAdjust    time.Time
	throttleCount int // number of rate reductions made during the crawl
}

// New creates an Engine. fetcher is used to retrieve pages (raw or headless); a separate
// raw fetcher is always used for robots.txt regardless of render mode.
func New(opts Options, fetcher Fetcher) *Engine {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	limit := rate.Inf
	if opts.RatePerSecond > 0 {
		limit = rate.Limit(opts.RatePerSecond)
	}
	e := &Engine{
		opts:     opts,
		fetcher:  fetcher,
		robots:   newRobotsManager(NewHTTPFetcher(opts), opts.UserAgent),
		limiter:  rate.NewLimiter(limit, 1),
		baseRate: opts.RatePerSecond,
	}
	// Gate every redirect hop the raw fetcher follows against the same scope/exclude/robots
	// check applied before a URL is ever enqueued, so a redirect can't escape crawl scope
	// mid-fetch. Only applies to the manual-hop HTTPFetcher; headless rendering follows
	// redirects inside the browser and isn't covered by this check.
	if hf, ok := fetcher.(*HTTPFetcher); ok {
		hf.allowRedirect = e.crawlable
	}
	return e
}

// logf writes a crawl progress line to stderr when verbose logging is enabled.
func (e *Engine) logf(format string, args ...any) {
	if !e.opts.Verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "[crawl] "+format+"\n", args...)
}

type task struct {
	url      string
	depth    int
	referrer string
}

// Crawl walks the site starting at seed and returns the collected Result.
func (e *Engine) Crawl(ctx context.Context, seed string) (*Result, error) {
	seed = normalizeURL(seed, e.opts.StripQuery)
	su, err := url.Parse(seed)
	if err != nil {
		return nil, err
	}
	e.seedHost = su.Host

	rateDesc := "unlimited"
	if e.opts.RatePerSecond > 0 {
		rateDesc = fmt.Sprintf("%.3g req/s", e.opts.RatePerSecond)
	}
	e.logf("starting crawl of %s (depth=%d, max-pages=%d, concurrency=%d, rate=%s, adaptive-delay=%t)",
		seed, e.opts.MaxDepth, e.opts.MaxPages, e.opts.Concurrency, rateDesc, e.opts.AdaptiveDelay)

	result := &Result{
		Seed:      seed,
		Robots:    make(map[string]*RobotsData),
		StartedAt: time.Now(),
		Opts:      e.opts,
		index:     make(map[string]*Page),
	}

	var (
		mu        sync.Mutex // guards visited, result.Pages, result.index, notCrawled, limit flags
		visited   = make(map[string]bool)
		wg        sync.WaitGroup
		sem       = make(chan struct{}, e.opts.Concurrency)
		pageCount int64

		// Coverage tracking: in-scope, robots-allowed URLs we discovered but declined to fetch
		// because a limit was reached, plus which limit it was. Reconciled against the crawled
		// set at the end (a URL reachable by a shorter path may still have been crawled).
		notCrawled        = make(map[string]bool)
		pageLimitReached  bool
		depthLimitReached bool
	)

	var enqueue func(t task)
	enqueue = func(t task) {
		norm := normalizeURL(t.url, e.opts.StripQuery)
		mu.Lock()
		if visited[norm] {
			mu.Unlock()
			return
		}
		visited[norm] = true
		mu.Unlock()

		u, perr := url.Parse(norm)
		if perr != nil || !e.crawlable(ctx, u) {
			return
		}
		if e.opts.MaxPages > 0 && atomic.AddInt64(&pageCount, 1) > int64(e.opts.MaxPages) {
			// This URL passed scope + robots + dedup but exceeds the page budget: a genuine,
			// reportable coverage gap.
			mu.Lock()
			notCrawled[norm] = true
			pageLimitReached = true
			mu.Unlock()
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			if err := e.limiter.Wait(ctx); err != nil {
				return
			}

			page, _ := e.fetcher.Fetch(ctx, norm)
			if page == nil {
				return
			}
			if page.Err != "" {
				e.logf("error fetching %s (depth=%d): %s", norm, t.depth, page.Err)
			} else {
				e.logf("fetched %s -> %d (%s, depth=%d)", norm, page.StatusCode, page.Duration.Round(time.Millisecond), t.depth)
			}
			if page.StatusCode == http.StatusTooManyRequests || page.StatusCode == http.StatusServiceUnavailable {
				e.throttleAfter429(page)
			}
			page.Depth = t.depth
			page.Referrer = t.referrer
			page.Links = e.extractLinks(page)

			mu.Lock()
			result.Pages = append(result.Pages, page)
			result.index[normalizeURL(page.RequestedURL, e.opts.StripQuery)] = page
			if page.FinalURL != "" {
				result.index[normalizeURL(page.FinalURL, e.opts.StripQuery)] = page
			}
			mu.Unlock()

			// MaxDepth == 0 means unlimited: the crawl is bounded by the page budget instead.
			atDepthLimit := e.opts.MaxDepth > 0 && t.depth >= e.opts.MaxDepth
			for _, link := range page.Links {
				if link.External && !e.opts.FollowExternal {
					continue
				}
				if link.Nofollow && !e.opts.FollowNofollow {
					continue
				}
				if atDepthLimit {
					// We won't follow links past the depth limit. Record in-scope, allowed
					// targets as a coverage gap so the report can flag partial coverage.
					ln := normalizeURL(link.URL, e.opts.StripQuery)
					if lu, err := url.Parse(ln); err == nil && e.crawlable(ctx, lu) {
						mu.Lock()
						notCrawled[ln] = true
						depthLimitReached = true
						mu.Unlock()
					}
					continue
				}
				enqueue(task{url: link.URL, depth: t.depth + 1, referrer: page.FinalURL})
			}
		}()
	}

	enqueue(task{url: seed, depth: 0})
	wg.Wait()

	e.backoffMu.Lock()
	result.ThrottleEvents = e.throttleCount
	result.FinalRate = e.curRate
	e.backoffMu.Unlock()

	// Reconcile the discovered-but-declined set against what actually got crawled: a URL we
	// declined on one path may have been reached by a shorter one. Whatever remains is a real
	// coverage gap.
	uncrawled := 0
	for u := range notCrawled {
		if _, ok := result.index[u]; !ok {
			uncrawled++
		}
	}
	result.Coverage = Coverage{
		Complete:             uncrawled == 0,
		DiscoveredNotCrawled: uncrawled,
		PageLimitReached:     pageLimitReached && uncrawled > 0,
		DepthLimitReached:    depthLimitReached && uncrawled > 0,
		MaxPages:             e.opts.MaxPages,
		MaxDepth:             e.opts.MaxDepth,
	}

	result.Finished = time.Now()
	e.collectRobots(ctx, result)
	return result, ctx.Err()
}

// throttleAfter429 slows the crawl after the server signals it is overloaded (HTTP 429 or
// 503). It halves the effective requests-per-second on each trigger, never going faster than
// any Retry-After header asks for and never slower than backoffMinRate. Triggers arriving
// within backoffDebounce of the last adjustment are ignored so a burst of concurrent 429s
// backs the crawl off once rather than collapsing straight to the floor.
func (e *Engine) throttleAfter429(page *Page) {
	if !e.opts.AdaptiveDelay {
		return
	}
	e.backoffMu.Lock()
	defer e.backoffMu.Unlock()

	now := time.Now()
	if !e.lastAdjust.IsZero() && now.Sub(e.lastAdjust) < backoffDebounce {
		return
	}
	e.lastAdjust = now

	prev := e.curRate
	if prev <= 0 {
		prev = e.baseRate
	}
	var next float64
	if prev <= 0 {
		next = backoffStartRate // crawl was previously unrestricted
	} else {
		next = prev / 2
	}
	// The floor only bounds our halving heuristic.
	if next < backoffMinRate {
		next = backoffMinRate
	}
	// An explicit Retry-After is a direct server instruction, so honor it even below the
	// heuristic floor: never crawl faster than it asks.
	if ra := retryAfterSeconds(page.Header); ra > 0 {
		if byRetry := 1.0 / ra; byRetry < next {
			next = byRetry
		}
	}
	e.curRate = next
	e.throttleCount++
	e.limiter.SetLimit(rate.Limit(next))

	target := page.FinalURL
	if target == "" {
		target = page.RequestedURL
	}
	e.logf("HTTP %d from %s — reducing crawl rate to %.3g req/s", page.StatusCode, target, next)
}

// retryAfterSeconds parses a Retry-After header, supporting both the delay-seconds and
// HTTP-date forms. It returns 0 when the header is absent, unparseable, or in the past.
func retryAfterSeconds(h http.Header) float64 {
	if h == nil {
		return 0
	}
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return float64(secs)
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t).Seconds(); d > 0 {
			return d
		}
	}
	return 0
}

// crawlable reports whether u is eligible to be fetched: in scope and not robots-disallowed.
// It deliberately ignores depth/page limits and the visited set, so it can also classify a
// frontier link the crawl declined to follow (was it skipped because it's out of scope, or
// because we hit a limit?).
func (e *Engine) crawlable(ctx context.Context, u *url.URL) bool {
	if !e.inScope(u) {
		return false
	}
	if e.opts.RespectRobots && !e.robots.allowed(ctx, u) {
		return false
	}
	return true
}

// inScope reports whether u should be crawled given host scope and include/exclude rules.
func (e *Engine) inScope(u *url.URL) bool {
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if !e.opts.FollowExternal && !sameSite(e.seedHost, u.Host, e.opts.AllowSubdomains) {
		return false
	}
	s := u.String()
	for _, re := range e.opts.Exclude {
		if re.MatchString(s) {
			return false
		}
	}
	if len(e.opts.Include) > 0 {
		for _, re := range e.opts.Include {
			if re.MatchString(s) {
				return true
			}
		}
		return false
	}
	return true
}

// extractLinks pulls outbound links from a page's parsed document.
func (e *Engine) extractLinks(page *Page) []Link {
	if page.Doc == nil {
		return nil
	}
	base, err := url.Parse(page.FinalURL)
	if err != nil {
		return nil
	}
	// <base href> overrides the document URL as the resolution base for relative links.
	// Only the first base element with an href counts, matching HTML semantics; not scoped
	// to <head> for parser leniency (a stray <base> outside <head> is still honored). A
	// relative base href is itself resolved against the document URL.
	if href, ok := page.Doc.Find("base[href]").First().Attr("href"); ok {
		if href = strings.TrimSpace(href); href != "" {
			if b, berr := url.Parse(href); berr == nil {
				base = base.ResolveReference(b)
			}
		}
	}
	seen := make(map[string]bool)
	var links []Link
	page.Doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		abs, resolved, ok := resolveURL(base, href, e.opts.StripQuery)
		if !ok || seen[abs] {
			return
		}
		seen[abs] = true
		rel, _ := s.Attr("rel")
		lu, perr := url.Parse(abs)
		external := true
		if perr == nil {
			external = !sameSite(e.seedHost, lu.Host, e.opts.AllowSubdomains)
		}
		links = append(links, Link{
			URL:      abs,
			Resolved: resolved,
			Anchor:   strings.TrimSpace(s.Text()),
			Rel:      rel,
			Nofollow: strings.Contains(strings.ToLower(rel), "nofollow"),
			External: external,
		})
	})
	return links
}

// collectRobots ensures Result.Robots has an entry for every host that was crawled, so
// the robots analyzer has data even when RespectRobots is disabled.
func (e *Engine) collectRobots(ctx context.Context, result *Result) {
	hosts := make(map[string]*url.URL)
	for _, p := range result.Pages {
		ref := p.FinalURL
		if ref == "" {
			ref = p.RequestedURL
		}
		if u, err := url.Parse(ref); err == nil && u.Host != "" {
			if _, ok := hosts[u.Host]; !ok {
				hosts[u.Host] = u
			}
		}
	}
	for host, u := range hosts {
		result.Robots[host] = e.robots.get(ctx, u)
	}
}
