package runner

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/sitemap"
	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// sitemapSet is the outcome of the sitemap pre-pass: everything the sitemap declared (handed
// to the sitemap analyzer unchanged) and the in-scope subset, keyed by the crawl's own URL
// normalization, that progress and Coverage count against. Read-only once built.
type sitemapSet struct {
	all     sitemap.Discovery
	inScope map[string]bool
}

// discoverSitemap runs the pre-pass: robots.txt Sitemap: lines for the seed host, then the
// conventional paths, through fetcher (the governed analyzer fetcher, so it obeys the crawl's
// rate limit and robots.txt). It never fails: no sitemap is an empty set.
func discoverSitemap(ctx context.Context, engine *crawler.Engine, fetcher crawler.Fetcher, seed string) sitemapSet {
	robots := map[string]*crawler.RobotsData{}
	// RobotsFor with a canceled context would cache an empty robots entry for the host that
	// the crawl then reuses, so skip the pre-pass entirely once ctx is done.
	if ctx.Err() != nil {
		return sitemapSet{all: sitemap.Discovery{URLs: map[string]bool{}}, inScope: map[string]bool{}}
	}
	if rd := engine.RobotsFor(ctx, seed); rd != nil {
		robots[rd.Host] = rd
	}
	d := sitemap.DiscoverAll(ctx, fetcher, seed, robots)
	in := make(map[string]bool, len(d.URLs))
	for u := range d.URLs {
		if engine.InScope(u) {
			in[engine.Normalize(u)] = true
		}
	}
	return sitemapSet{all: d, inScope: in}
}

// found reports whether any sitemap parsed.
func (s sitemapSet) found() bool { return s.all.Parsed > 0 }

// match reports whether a recorded page (by requested or final URL) is an in-scope sitemap
// URL, returning the matched key so callers can count each sitemap URL once.
func (s sitemapSet) match(requested, final string, normalize func(string) string) (string, bool) {
	if len(s.inScope) == 0 {
		return "", false
	}
	for _, u := range []string{final, requested} {
		if u == "" {
			continue
		}
		if k := normalize(u); s.inScope[k] {
			return k, true
		}
	}
	return "", false
}

// fetchedCount counts the distinct in-scope sitemap URLs among pages.
func (s sitemapSet) fetchedCount(pages []*crawler.Page, normalize func(string) string) int {
	seen := map[string]bool{}
	for _, p := range pages {
		if p == nil {
			continue
		}
		if k, ok := s.match(p.RequestedURL, p.FinalURL, normalize); ok {
			seen[k] = true
		}
	}
	return len(seen)
}

// budgetNote is the Report.Notes entry for a page budget smaller than the sitemap, or "".
func budgetNote(maxPages, sitemapTotal int) string {
	if maxPages <= 0 || sitemapTotal <= 0 || maxPages >= sitemapTotal {
		return ""
	}
	pct := maxPages * 100 / sitemapTotal
	if pct < 1 {
		pct = 1
	}
	return fmt.Sprintf("The page budget (%s) is below the sitemap total (%s); this crawl covered at most %d%% of the sitemap.",
		groupThousands(maxPages), groupThousands(sitemapTotal), pct)
}

// groupThousands formats n with comma thousands separators (1240 -> "1,240").
func groupThousands(n int) string {
	s := strconv.Itoa(n)
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return sign + b.String()
}

// SitemapPreview is what the sitemap pre-pass would find for a crawl, without crawling.
type SitemapPreview struct {
	Found     bool     `json:"found"`
	Total     int      `json:"total"`   // in-scope sitemap URLs
	Sources   []string `json:"sources"` // sitemap files that parsed, sorted; never null
	ViaRobots bool     `json:"via_robots"`
}

// PreviewSitemap runs only the sitemap discovery step for cfg and seed: the same robots.txt
// handling, User-Agent, rate limit and scope rules a crawl would apply. Errors are config
// errors (bad regex, bad seed); a missing sitemap is Found == false.
func PreviewSitemap(ctx context.Context, cfg config.Config, seed string) (SitemapPreview, error) {
	if seed == "" {
		seed = cfg.Seed
	}
	if err := cfg.Validate(); err != nil {
		return SitemapPreview{}, err
	}
	opts, err := cfg.ToOptions()
	if err != nil {
		return SitemapPreview{}, err
	}
	engine := crawler.New(opts, crawler.NewHTTPFetcher(opts))
	seed, err = engine.SetSeed(seed)
	if err != nil {
		return SitemapPreview{}, err
	}
	f := crawler.NewHTTPFetcher(opts)
	if u, perr := url.Parse(seed); perr == nil {
		f.RestrictCredentialsToHost(u.Host, opts.AllowSubdomains)
	}
	sm := discoverSitemap(ctx, engine, engine.Govern(f), seed)
	sources := sm.all.Sources
	if sources == nil {
		sources = []string{}
	}
	return SitemapPreview{Found: sm.found(), Total: len(sm.inScope), Sources: sources, ViaRobots: sm.all.ViaRobots}, nil
}
