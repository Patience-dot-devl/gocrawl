package crawler

import (
	"context"
	"net/url"
	"sync"

	"github.com/temoto/robotstxt"
)

// robotsManager lazily fetches and caches robots.txt per host.
type robotsManager struct {
	fetcher   Fetcher
	userAgent string

	mu    sync.Mutex
	cache map[string]*RobotsData
}

func newRobotsManager(fetcher Fetcher, userAgent string) *robotsManager {
	return &robotsManager{
		fetcher:   fetcher,
		userAgent: userAgent,
		cache:     make(map[string]*RobotsData),
	}
}

// get returns the (cached) robots.txt data for the host of u, fetching it if necessary.
func (m *robotsManager) get(ctx context.Context, u *url.URL) *RobotsData {
	host := u.Host
	m.mu.Lock()
	if d, ok := m.cache[host]; ok {
		m.mu.Unlock()
		return d
	}
	m.mu.Unlock()

	data := m.fetch(ctx, u)

	m.mu.Lock()
	m.cache[host] = data
	m.mu.Unlock()
	return data
}

func (m *robotsManager) fetch(ctx context.Context, u *url.URL) *RobotsData {
	robotsURL := u.Scheme + "://" + u.Host + "/robots.txt"
	data := &RobotsData{Host: u.Host}

	page, err := m.fetcher.Fetch(ctx, robotsURL)
	if err != nil || page == nil {
		return data
	}
	data.Status = page.StatusCode
	if page.StatusCode != 200 || len(page.Body) == 0 {
		return data
	}
	parsed, perr := robotstxt.FromBytes(page.Body)
	if perr != nil {
		return data
	}
	data.Found = true
	data.data = parsed
	data.Sitemaps = parsed.Sitemaps
	return data
}

// allowed reports whether u may be crawled per robots.txt for the configured user agent.
func (m *robotsManager) allowed(ctx context.Context, u *url.URL) bool {
	d := m.get(ctx, u)
	path := u.Path
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return d.TestAgent(path, m.userAgent)
}
