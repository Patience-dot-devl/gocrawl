package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Engine crawls a website concurrently within the configured scope.
type Engine struct {
	opts     Options
	fetcher  Fetcher
	robots   *robotsManager
	limiter  *AdaptiveLimiter
	seedHost string
}

// New creates an Engine. fetcher is used to retrieve pages (raw or headless); a separate
// raw fetcher is always used for robots.txt regardless of render mode.
func New(opts Options, fetcher Fetcher) *Engine {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	e := &Engine{
		opts:    opts,
		fetcher: fetcher,
		// NewUAPool(opts).Default() is the UA actually sent when UserAgents rotation is
		// configured — opts.UserAgent alone would test robots.txt against an identity the
		// crawler never sends once a pool supersedes it.
		robots:  newRobotsManager(NewHTTPFetcher(opts), NewUAPool(opts).Default()),
		limiter: NewAdaptiveLimiter(opts.RatePerSecond, opts.AdaptiveDelay),
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
	if e.opts.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.opts.MaxDuration)
		defer cancel()
	}

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
	durationDesc := "unlimited"
	if e.opts.MaxDuration > 0 {
		durationDesc = e.opts.MaxDuration.String()
	}
	e.logf("starting crawl of %s (depth=%d, max-pages=%d, concurrency=%d, rate=%s, max-duration=%s, adaptive-delay=%t)",
		seed, e.opts.MaxDepth, e.opts.MaxPages, e.opts.Concurrency, rateDesc, durationDesc, e.opts.AdaptiveDelay)

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
		pageCount int64

		// Coverage tracking: in-scope, robots-allowed URLs we discovered but declined to fetch
		// because a limit was reached, plus which limit it was. Reconciled against the crawled
		// set at the end (a URL reachable by a shorter path may still have been crawled).
		notCrawled        = make(map[string]bool)
		pageLimitReached  bool
		depthLimitReached bool
	)

	fr := newFrontier()
	stopWatching := fr.watchCancellation(ctx)
	defer stopWatching()

	// enqueue applies dedup, scope/robots, and the page-budget check, then pushes an eligible
	// task onto the frontier for a worker to pick up. Called for the seed and for every link
	// a worker discovers while processing a task.
	enqueue := func(t task) {
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
		t.url = norm
		fr.push(t)
	}

	// process fetches and analyzes one task, then enqueues its children. taskDone is deferred
	// so it fires (marking the task no longer pending) only after any children are already
	// pushed — otherwise the frontier could see pending hit zero while a child is about to
	// arrive.
	process := func(t task) {
		defer fr.taskDone()

		if err := e.limiter.Wait(ctx); err != nil {
			return
		}

		page, _ := e.fetcher.Fetch(ctx, t.url)
		if page == nil {
			return
		}
		if page.Err != "" {
			e.logf("error fetching %s (depth=%d): %s", t.url, t.depth, page.Err)
		} else {
			e.logf("fetched %s -> %d (%s, depth=%d)", t.url, page.StatusCode, page.Duration.Round(time.Millisecond), t.depth)
		}
		if adjusted, next := e.limiter.OnResponse(page.StatusCode, page.Header); adjusted {
			target := page.FinalURL
			if target == "" {
				target = page.RequestedURL
			}
			e.logf("HTTP %d from %s — reducing crawl rate to %.3g req/s", page.StatusCode, target, next)
		}
		page.Depth = t.depth
		page.Referrer = t.referrer
		page.Links = e.extractLinks(page)

		var finalNorm string
		if page.FinalURL != "" {
			finalNorm = normalizeURL(page.FinalURL, e.opts.StripQuery)
		}

		mu.Lock()
		if finalNorm != "" && result.index[finalNorm] != nil {
			// A concurrent fetch already recorded this exact final URL — e.g. two links
			// discovered on the same page, one a redirect and one pointing directly at
			// the redirect's destination, enqueued before either fetch completed. Drop
			// this duplicate rather than double-reporting every per-page analyzer issue
			// for the same content.
			mu.Unlock()
			return
		}
		result.Pages = append(result.Pages, page)
		result.index[normalizeURL(page.RequestedURL, e.opts.StripQuery)] = page
		if finalNorm != "" {
			result.index[finalNorm] = page
			// Mark the redirect's destination visited too, so a separate link pointing
			// directly at it doesn't trigger a second fetch (and duplicate per-page
			// issues) for content already covered by this page.
			visited[finalNorm] = true
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
	}

	// Seed the frontier before starting any worker: next() treats "nothing queued and
	// nothing pending" as exhaustion, so a worker started first could see an empty frontier
	// and exit before the seed ever arrived.
	enqueue(task{url: seed, depth: 0})

	// A fixed pool of workers pulls from the frontier, so the goroutine count is bounded by
	// Concurrency regardless of how large the frontier grows — unlike spawning a goroutine
	// per discovered URL, which scales with the frontier instead of with concurrency.
	var wg sync.WaitGroup
	wg.Add(e.opts.Concurrency)
	for range e.opts.Concurrency {
		go func() {
			defer wg.Done()
			for {
				t, ok := fr.next(ctx)
				if !ok {
					return
				}
				process(t)
			}
		}()
	}

	wg.Wait()

	result.ThrottleEvents = e.limiter.ThrottleCount()
	result.FinalRate = e.limiter.CurrentRate()

	// Reconcile the discovered-but-declined set against what actually got crawled: a URL we
	// declined on one path may have been reached by a shorter one. Whatever remains is a real
	// coverage gap.
	uncrawled := 0
	for u := range notCrawled {
		if _, ok := result.index[u]; !ok {
			uncrawled++
		}
	}
	interrupted := ctx.Err() != nil
	// A deadline (--max-duration) and an external cancellation (e.g. Ctrl-C) both stop the
	// crawl via the same context-cancellation path, but are worth reporting distinctly.
	durationLimitReached := errors.Is(ctx.Err(), context.DeadlineExceeded)
	result.Coverage = Coverage{
		Complete:             uncrawled == 0 && !interrupted,
		DiscoveredNotCrawled: uncrawled,
		PageLimitReached:     pageLimitReached && uncrawled > 0,
		DepthLimitReached:    depthLimitReached && uncrawled > 0,
		Interrupted:          interrupted,
		DurationLimitReached: durationLimitReached,
		MaxPages:             e.opts.MaxPages,
		MaxDepth:             e.opts.MaxDepth,
	}

	result.Finished = time.Now()
	e.collectRobots(ctx, result)
	// A canceled context (e.g. an operator's Ctrl-C) stops the crawl early rather than
	// failing it: whatever was fetched before cancellation is a legitimate partial result,
	// reported honestly via Coverage.Interrupted rather than discarded by returning an error.
	return result, nil
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
