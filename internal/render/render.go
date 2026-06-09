// Package render provides the headless-rendering Fetcher. Headless rendering via chromedp
// (and real Core Web Vitals) is on the roadmap; for now this fetcher performs a raw HTTP
// fetch and annotates each page noting that JS rendering is not yet active, so crawls with
// --render headless still work end-to-end.
package render

import (
	"context"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// HeadlessFetcher is a placeholder headless renderer. It delegates to a raw HTTP fetch and
// marks the page's RenderResult as not implemented.
type HeadlessFetcher struct {
	inner crawler.Fetcher
}

// NewHeadlessFetcher returns a headless fetcher built from the given crawl options.
func NewHeadlessFetcher(opts crawler.Options) *HeadlessFetcher {
	return &HeadlessFetcher{inner: crawler.NewHTTPFetcher(opts)}
}

// Fetch delegates to the raw fetcher and annotates the render status.
func (h *HeadlessFetcher) Fetch(ctx context.Context, rawURL string) (*crawler.Page, error) {
	page, err := h.inner.Fetch(ctx, rawURL)
	if page != nil {
		page.Render = &crawler.RenderResult{
			Implemented: false,
			Note:        "headless rendering not yet implemented; served via raw fetch",
		}
	}
	return page, err
}
