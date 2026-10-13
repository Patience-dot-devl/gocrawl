package runner

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

func TestRunWithProgressReportsPhasesAndSitemapCounts(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)

	var (
		mu     sync.Mutex
		events []Progress
	)
	rep, err := Run(context.Background(), config.Default(), ts.URL, WithProgress(func(p Progress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 || events[0].Phase != PhaseSitemap {
		t.Fatalf("first event = %+v, want phase %q", events, PhaseSitemap)
	}
	order := map[Phase]int{PhaseSitemap: 0, PhaseCrawling: 1, PhaseAnalyzing: 2}
	pages := map[int]bool{}
	for i, ev := range events {
		if i > 0 && order[ev.Phase] < order[events[i-1].Phase] {
			t.Errorf("phase went backwards at event %d: %s after %s", i, ev.Phase, events[i-1].Phase)
		}
		if ev.Fetched > 0 {
			pages[ev.Fetched] = true
		}
		if strings.HasSuffix(ev.CurrentURL, "/off") && ev.InSitemap {
			t.Errorf("/off is not in the sitemap but was reported in_sitemap")
		}
		if strings.HasSuffix(ev.CurrentURL, "/a") && !ev.InSitemap {
			t.Errorf("/a is in the sitemap but was reported off it")
		}
	}
	if len(pages) != rep.PagesCrawled {
		t.Errorf("saw %d distinct Fetched values, want one per page (%d)", len(pages), rep.PagesCrawled)
	}
	last := events[len(events)-1]
	if last.Phase != PhaseAnalyzing || last.Fetched != rep.PagesCrawled {
		t.Errorf("last event = phase %s fetched %d, want analyzing / %d", last.Phase, last.Fetched, rep.PagesCrawled)
	}
	if !last.SitemapFound || last.SitemapTotal != 3 {
		t.Errorf("last event sitemap = found %v total %d, want true / 3", last.SitemapFound, last.SitemapTotal)
	}
	if last.SitemapFetched != rep.Coverage.SitemapFetched || last.SitemapFetched != 3 {
		t.Errorf("SitemapFetched = %d, Coverage says %d, want 3", last.SitemapFetched, rep.Coverage.SitemapFetched)
	}
	if last.OffSitemap != 1 {
		t.Errorf("OffSitemap = %d, want 1 (/off)", last.OffSitemap)
	}
}

func TestRunWithoutProgressOptionSendsNothing(t *testing.T) {
	var hits atomic.Int32
	ts := sitemapSite(t, &hits)
	// Nothing to assert beyond "no panic on the nil reporter": the CLI path.
	if _, err := Run(context.Background(), config.Default(), ts.URL); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
