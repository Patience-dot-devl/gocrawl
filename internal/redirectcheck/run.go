package redirectcheck

import (
	"context"
	"sync"

	"golang.org/x/time/rate"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// RunOptions configures a check-redirects run.
type RunOptions struct {
	Domain        string
	SitemapURL    string
	Fetcher       crawler.Fetcher
	Concurrency   int
	RatePerSecond float64
}

// Run classifies every rule, fetches the site's sitemap once, then checks each in-scope rule
// concurrently (bounded by opts.Concurrency and opts.RatePerSecond). Results are returned in
// the same order as rules. Rows classified external or dynamic-pattern are not fetched.
func Run(ctx context.Context, rules []Rule, opts RunOptions) ([]RowResult, error) {
	sitemapURLs, err := DiscoverSitemap(ctx, opts.Fetcher, opts.Domain, opts.SitemapURL)
	if err != nil {
		return nil, err
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	var limiter *rate.Limiter
	if opts.RatePerSecond > 0 {
		limiter = rate.NewLimiter(rate.Limit(opts.RatePerSecond), 1)
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
			if limiter != nil {
				_ = limiter.Wait(ctx)
			}
			results[i] = CheckRule(ctx, opts.Fetcher, opts.Domain, rule, sitemapURLs)
		}(i, rule)
	}
	wg.Wait()
	return results, nil
}
