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

// progressReporter turns engine events into runner.Progress for WithProgress (fn) and records
// the crawl tape Run saves in Coverage. Run always creates one; fn is nil without WithProgress.
type progressReporter struct {
	fn func(Progress)

	mu        sync.Mutex // guards everything below; fn is called with it held to keep order
	phase     Phase
	last      crawler.Progress
	set       sitemapSet
	normalize func(string) string
	matched   map[string]bool
	inSitemap bool
	// tape and tapeOff grow by one character per page event (TapeChar, tapeOffChar).
	tape, tapeOff []byte
}

func newProgressReporter(fn func(Progress)) *progressReporter {
	return &progressReporter{fn: fn, phase: PhaseSitemap, matched: map[string]bool{}}
}

// setPhase records a phase change and reports it.
func (r *progressReporter) setPhase(p Phase) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phase = p
	r.emitLocked()
}

// setSitemap hands over the pre-pass result. Called once, before the crawl starts.
func (r *progressReporter) setSitemap(set sitemapSet, normalize func(string) string) {
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
		r.tape = append(r.tape, crawler.TapeChar(p.Status))
		r.tapeOff = append(r.tapeOff, tapeOffChar(r.set.found(), r.inSitemap))
	}
	r.last = p
	r.emitLocked()
}

func (r *progressReporter) emitLocked() {
	if r.fn == nil {
		return
	}
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

// tapes returns the tape recorded so far.
func (r *progressReporter) tapes() (tape, off string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.tape), string(r.tapeOff)
}

// tapeOffChar is the tape_off character for one page: '1' when a sitemap was found and the
// page is not in it, '0' otherwise (without a sitemap no page is "off" it).
func tapeOffChar(sitemapFound, inSitemap bool) byte {
	if sitemapFound && !inSitemap {
		return '1'
	}
	return '0'
}

// TapeOffChar is the tape_off character for the page this event reports. The web server
// builds the live tape_off with it; Run builds Coverage.TapeOff with the same rule.
func (p Progress) TapeOffChar() byte {
	return tapeOffChar(p.SitemapFound, p.InSitemap)
}
