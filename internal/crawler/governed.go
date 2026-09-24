package crawler

import (
	"context"
	"errors"
	"net/url"
)

// ErrRobotsDisallowed is returned by a governed fetcher for a URL robots.txt forbids.
var ErrRobotsDisallowed = errors.New("disallowed by robots.txt")

// Govern wraps inner so every fetch through it obeys the crawl's own politeness rules: it
// waits on the engine's rate limiter, refuses URLs robots.txt disallows when RespectRobots is
// set, and reports each response back to the adaptive limiter so a 429 seen here slows the
// whole run. Analyzers that fetch extra resources after the crawl (sitemap.xml, llms.txt, the
// --specialized WordPress and Shopify probes) are handed a governed fetcher by runner.Run;
// without it those requests would bypass both the rate limit and robots.txt the crawl itself
// honoured, and run at full speed the moment the crawl finished.
//
// Scope is deliberately not enforced: a sitemap declared in robots.txt routinely lives on
// another host, and that is the caller's decision, not this wrapper's.
func (e *Engine) Govern(inner Fetcher) Fetcher {
	return &governedFetcher{inner: inner, engine: e}
}

type governedFetcher struct {
	inner  Fetcher
	engine *Engine
}

func (g *governedFetcher) Fetch(ctx context.Context, rawURL string) (*Page, error) {
	if g.engine.opts.RespectRobots {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		if !g.engine.robots.allowed(ctx, u) {
			return nil, ErrRobotsDisallowed
		}
	}
	if err := g.engine.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	page, err := g.inner.Fetch(ctx, rawURL)
	if page != nil {
		g.engine.limiter.OnResponse(page.StatusCode, page.Header)
	}
	return page, err
}
