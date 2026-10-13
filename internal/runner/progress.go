package runner

import (
	"sync"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Phase is the stage a Run is in.
type Phase string

const (
	PhaseSitemap   Phase = "sitemap"   // discovery pre-pass, before the crawl
	PhaseCrawling  Phase = "crawling"  // Engine.Crawl is running
	PhaseAnalyzing Phase = "analyzing" // analyzers and report building
)

// Progress is crawler.Progress plus the run phase and progress against the site's sitemap.
type Progress struct {
	crawler.Progress
	Phase Phase `json:"phase"`
	// SitemapFound is true when the pre-pass parsed a sitemap. SitemapTotal counts its
	// in-scope URLs; SitemapFetched counts the distinct ones fetched so far.
	SitemapFound   bool `json:"sitemap_found"`
	SitemapTotal   int  `json:"sitemap_total"`
	SitemapFetched int  `json:"sitemap_fetched"`
	// OffSitemap is Fetched - SitemapFetched: fetched pages the sitemap doesn't list.
	OffSitemap int `json:"off_sitemap"`
	// InSitemap reports whether CurrentURL (or where it redirected) is in the sitemap.
	InSitemap bool `json:"in_sitemap"`
}

// progressReporter turns engine events into runner.Progress. A nil reporter (no WithProgress)
// ignores every call, so Run needs no conditionals.
type progressReporter struct {
	fn func(Progress)

	mu        sync.Mutex // guards everything below; fn is called with it held to keep order
	phase     Phase
	last      crawler.Progress
	set       sitemapSet
	normalize func(string) string
	matched   map[string]bool
	inSitemap bool
}

func newProgressReporter(fn func(Progress)) *progressReporter {
	if fn == nil {
		return nil
	}
	return &progressReporter{fn: fn, phase: PhaseSitemap, matched: map[string]bool{}}
}

// setPhase records a phase change and reports it.
func (r *progressReporter) setPhase(p Phase) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phase = p
	r.emitLocked()
}

// setSitemap hands over the pre-pass result. Called once, before the crawl starts.
func (r *progressReporter) setSitemap(set sitemapSet, normalize func(string) string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.set, r.normalize = set, normalize
}

// onEngine is the crawler.Options.OnProgress callback. A rise in Fetched marks a page event;
// anything else is a state event (pause, resume) that keeps the last page's sitemap flag.
func (r *progressReporter) onEngine(p crawler.Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.Fetched > r.last.Fetched {
		r.inSitemap = false
		if key, ok := r.set.match(p.CurrentURL, p.FinalURL, r.normalize); ok {
			r.matched[key] = true
			r.inSitemap = true
		}
	}
	r.last = p
	r.emitLocked()
}

func (r *progressReporter) emitLocked() {
	fetched := len(r.matched)
	r.fn(Progress{
		Progress:       r.last,
		Phase:          r.phase,
		SitemapFound:   r.set.found(),
		SitemapTotal:   len(r.set.inScope),
		SitemapFetched: fetched,
		OffSitemap:     r.last.Fetched - fetched,
		InSitemap:      r.inSitemap,
	})
}
