package redirectcheck

import (
	"time"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// DefaultTimeout is the per-request timeout of a redirect check.
const DefaultTimeout = 15 * time.Second

// DefaultUserAgent is the User-Agent a redirect check sends unless told otherwise.
const DefaultUserAgent = "gocrawl/0.1 (+https://github.com/Patience-dot-devl/gocrawl)"

// NewFetcher builds the HTTP fetcher a redirect check runs on. It follows up to 10 redirects,
// recording each hop, and reads at most 5 MiB per body. It has no crawl scope or credentials:
// a redirect check fetches exactly the URLs in the rules and the sitemap. The CLI
// (`gocrawl check-redirects`) and the web API share it so the two behave the same.
func NewFetcher(timeout time.Duration, userAgent string) *crawler.HTTPFetcher {
	return crawler.NewHTTPFetcher(crawler.Options{
		Timeout:      timeout,
		UserAgent:    userAgent,
		MaxRedirects: 10,
		MaxBodyBytes: 5 << 20,
	})
}
