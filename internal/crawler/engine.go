package crawler

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/time/rate"
)

// Engine crawls a website concurrently within the configured scope.
type Engine struct {
	opts     Options
	fetcher  Fetcher
	robots   *robotsManager
	limiter  *rate.Limiter
	seedHost string
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
	return &Engine{
		opts:    opts,
		fetcher: fetcher,
		robots:  newRobotsManager(NewHTTPFetcher(opts), opts.UserAgent),
		limiter: rate.NewLimiter(limit, 1),
	}
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

	result := &Result{
		Seed:      seed,
		Robots:    make(map[string]*RobotsData),
		StartedAt: time.Now(),
		Opts:      e.opts,
		index:     make(map[string]*Page),
	}

	var (
		mu        sync.Mutex // guards visited, result.Pages, result.index
		visited   = make(map[string]bool)
		wg        sync.WaitGroup
		sem       = make(chan struct{}, e.opts.Concurrency)
		pageCount int64
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
		if perr != nil || !e.inScope(u) {
			return
		}
		if e.opts.RespectRobots && !e.robots.allowed(ctx, u) {
			return
		}
		if e.opts.MaxPages > 0 && atomic.AddInt64(&pageCount, 1) > int64(e.opts.MaxPages) {
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

			if t.depth >= e.opts.MaxDepth {
				return
			}
			for _, link := range page.Links {
				if link.External && !e.opts.FollowExternal {
					continue
				}
				if link.Nofollow && !e.opts.FollowNofollow {
					continue
				}
				enqueue(task{url: link.URL, depth: t.depth + 1, referrer: page.FinalURL})
			}
		}()
	}

	enqueue(task{url: seed, depth: 0})
	wg.Wait()

	result.Finished = time.Now()
	e.collectRobots(ctx, result)
	return result, ctx.Err()
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
	seen := make(map[string]bool)
	var links []Link
	page.Doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		abs, ok := resolveURL(base, href, e.opts.StripQuery)
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
