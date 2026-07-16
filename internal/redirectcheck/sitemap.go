package redirectcheck

import (
	"context"
	"fmt"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/sitemap"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// DiscoverSitemap fetches and parses the sitemap for domain, returning the normalized set of
// <loc> URLs it declares. If override is non-empty it is used as the only candidate;
// otherwise the conventional /sitemap.xml and /sitemap_index.xml locations are tried. An
// error is returned if nothing usable is found at any candidate — callers should treat this
// as fatal, since sitemap-membership columns would otherwise be silently wrong for every row.
func DiscoverSitemap(ctx context.Context, fetcher crawler.Fetcher, domain, override string) (map[string]bool, error) {
	candidates := map[string]bool{}
	where := domain + "'s default sitemap locations (/sitemap.xml, /sitemap_index.xml)"
	if override != "" {
		candidates[override] = true
		where = override
	} else {
		candidates["https://"+domain+"/sitemap.xml"] = false
		candidates["https://"+domain+"/sitemap_index.xml"] = false
	}

	rawURLs, parsed, _, truncated := sitemap.Fetch(ctx, fetcher, candidates)
	if parsed == 0 {
		if len(truncated) > 0 {
			return nil, fmt.Errorf("sitemap at %s exceeds the fetch size limit and was cut off before it could be parsed; raise crawl.max_body_bytes and retry", where)
		}
		return nil, fmt.Errorf("could not fetch/parse a sitemap at %s; pass --sitemap-url to point at the right location", where)
	}

	urls := make(map[string]bool, len(rawURLs))
	for u := range rawURLs {
		urls[normalizeForSitemap(u)] = true
	}
	return urls, nil
}
