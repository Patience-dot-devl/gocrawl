package redirectcheck

import (
	"context"
	"sync"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// RunOptions configures a check-redirects run.
type RunOptions struct {
	Domain        string
	SitemapURL    string
	Fetcher       crawler.Fetcher
	Concurrency   int
	RatePerSecond float64
	// AdaptiveDelay automatically slows requests (halving requests-per-second, honoring any
	// Retry-After header) when the site responds with HTTP 429/503, the same way the crawl
	// engine does. Without it, a fixed concurrency/rate that's too aggressive for the target
	// can trip bot-mitigation (e.g. Cloudflare) partway through a run.
	AdaptiveDelay bool
	// Stats, if non-nil, is filled in with adaptive-delay activity once Run returns.
	Stats *RunStats
}

// RunStats reports adaptive-delay activity for a Run call.
type RunStats struct {
	ThrottleEvents int
	FinalRate      float64
}

// Run classifies every rule, fetches the site's sitemap once, then checks each in-scope rule
// concurrently (bounded by opts.Concurrency and opts.RatePerSecond). Every fetch — sitemap
// discovery included — goes through a shared AdaptiveLimiter, so a run that starts too
// aggressively for the target site backs itself off instead of hammering it into a
// bot-mitigation block. Results are returned in the same order as rules. Rows classified
// external or dynamic-pattern are not fetched.
func Run(ctx context.Context, rules []Rule, opts RunOptions) ([]RowResult, error) {
	limiter := crawler.NewAdaptiveLimiter(opts.RatePerSecond, opts.AdaptiveDelay)
	fetcher := &crawler.AdaptiveFetcher{Fetcher: opts.Fetcher, Limiter: limiter}

	sitemapURLs, err := DiscoverSitemap(ctx, fetcher, opts.Domain, opts.SitemapURL)
	if err != nil {
		return nil, err
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	results := make([]RowResult, len(rules))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, rule := range rules {
		scope, err := Classify(rule, opts.Domain)
		if err != nil {
			results[i] = RowResult{Verdict: VerdictError, Notes: []string{"could not classify rule: " + err.Error()}}
			continue
		}
		if scope == ScopeExternal {
			results[i] = RowResult{Scope: ScopeExternal, Verdict: VerdictSkippedExternal}
			continue
		}
		if scope == ScopeDynamic {
			results[i] = RowResult{Scope: ScopeDynamic, Verdict: VerdictSkippedDynamic}
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(i int, rule Rule) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = CheckRule(ctx, fetcher, opts.Domain, rule, sitemapURLs)
		}(i, rule)
	}
	wg.Wait()

	if opts.Stats != nil {
		opts.Stats.ThrottleEvents = limiter.ThrottleCount()
		opts.Stats.FinalRate = limiter.CurrentRate()
	}
	return results, nil
}
