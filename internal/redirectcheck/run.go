package redirectcheck

import (
	"context"
	"sync"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// RunOptions configures a check-redirects run.
type RunOptions struct {
	Domain     string
	SitemapURL string
	// SitemapHint is the remedy named in the "no sitemap found" error. Empty means
	// DefaultSitemapHint, which names the CLI flag.
	SitemapHint   string
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
	// OnRow, if non-nil, is called once for every rule as its row completes. The arguments are
	// the number of rows done so far and the total number of rules. Calls are serialized
	// (never concurrent), and done rises by one per call, ending at total. Rows skipped
	// without a fetch (external, dynamic pattern, unclassifiable) count too. OnRow is not
	// called when Run fails before checking rows (no sitemap). It runs on a worker goroutine
	// and holds up the other workers' progress reports, so keep it short.
	OnRow func(done, total int)
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

	hint := opts.SitemapHint
	if hint == "" {
		hint = DefaultSitemapHint
	}
	sitemapURLs, err := discoverSitemap(ctx, fetcher, opts.Domain, opts.SitemapURL, hint)
	if err != nil {
		return nil, err
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	// rowDone reports one finished row. Skipped rows finish on this goroutine, checked rows on
	// worker goroutines; the mutex makes the count and the callback one ordered sequence.
	var progressMu sync.Mutex
	done, total := 0, len(rules)
	rowDone := func() {
		if opts.OnRow == nil {
			return
		}
		progressMu.Lock()
		defer progressMu.Unlock()
		done++
		opts.OnRow(done, total)
	}

	results := make([]RowResult, len(rules))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, rule := range rules {
		scope, err := Classify(rule, opts.Domain)
		if err != nil {
			results[i] = RowResult{Verdict: VerdictError, Notes: []string{"could not classify rule: " + err.Error()}}
			rowDone()
			continue
		}
		if scope == ScopeExternal {
			results[i] = RowResult{Scope: ScopeExternal, Verdict: VerdictSkippedExternal}
			rowDone()
			continue
		}
		if scope == ScopeDynamic {
			results[i] = RowResult{Scope: ScopeDynamic, Verdict: VerdictSkippedDynamic}
			rowDone()
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(i int, rule Rule) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = CheckRule(ctx, fetcher, opts.Domain, rule, sitemapURLs)
			rowDone()
		}(i, rule)
	}
	wg.Wait()

	if opts.Stats != nil {
		opts.Stats.ThrottleEvents = limiter.ThrottleCount()
		opts.Stats.FinalRate = limiter.CurrentRate()
	}
	return results, nil
}
