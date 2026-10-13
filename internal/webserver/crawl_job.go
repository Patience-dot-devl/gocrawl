package webserver

import (
	"github.com/Patience-dot-devl/gocrawl/internal/report"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// crawlJob is the crawl-specific part of a Job: what the web API knows about one crawl
// beyond its lifecycle. Later features add their fields here (the pause control).
type crawlJob struct {
	Seed    string
	Report  *report.Report
	StoreID string // set once the report is saved, so history and job views agree on one ID
	// MaxPages is the crawl's page budget (0 = unlimited), shown next to live progress.
	MaxPages int
	// Progress is the latest snapshot. recordProgress replaces the pointer on every event and
	// never writes through it, so a shallow Job snapshot may keep reading the old value.
	Progress *runner.Progress
	// Tape holds one status-class char per recorded page ('2','3','4','5','x'); TapeOff holds
	// '1' where that page is not in the sitemap. Appended only under the job manager's lock.
	Tape    []byte
	TapeOff []byte
}

// JobView is the API representation of a crawl job.
type JobView struct {
	ID         string         `json:"id"`
	Seed       string         `json:"seed"`
	Status     string         `json:"status"`
	Err        string         `json:"error,omitempty"`
	StartedAt  string         `json:"started_at,omitempty"`
	FinishedAt string         `json:"finished_at,omitempty"`
	Persisted  bool           `json:"persisted"`
	Report     *report.Report `json:"report,omitempty"`
	Progress   *progressView  `json:"progress,omitempty"`
}

func jobView(j Job[crawlJob]) JobView {
	return JobView{
		ID:         j.ID,
		Seed:       j.Data.Seed,
		Status:     string(j.Status),
		Err:        j.Err,
		StartedAt:  formatTime(j.StartedAt),
		FinishedAt: formatTime(j.FinishedAt),
		Persisted:  j.Data.StoreID != "",
		Report:     j.Data.Report,
	}
}

// crawlListItem is the lightweight API representation used by the crawl list endpoint —
// summary counts only, not the full report.
type crawlListItem struct {
	ID           string         `json:"id"`
	Seed         string         `json:"seed"`
	Status       string         `json:"status"`
	Err          string         `json:"error,omitempty"`
	StartedAt    string         `json:"started_at,omitempty"`
	FinishedAt   string         `json:"finished_at,omitempty"`
	PagesCrawled int            `json:"pages_crawled,omitempty"`
	BySeverity   map[string]int `json:"by_severity,omitempty"`
	Persisted    bool           `json:"persisted"`
	// StatusClasses, SitemapTotal, SitemapFetched and TapePreview feed the list's mini tapes and
	// sitemap coverage. TapePreview is short (<= 160 chars, worst status wins); the full tape
	// is only in GET /api/crawls/{id}.
	StatusClasses  map[string]int `json:"status_classes,omitempty"`
	SitemapTotal   int            `json:"sitemap_total,omitempty"`
	SitemapFetched int            `json:"sitemap_fetched,omitempty"`
	TapePreview    string         `json:"tape_preview,omitempty"`
}

func jobListItem(j Job[crawlJob]) crawlListItem {
	item := crawlListItem{
		ID:         j.ID,
		Seed:       j.Data.Seed,
		Status:     string(j.Status),
		Err:        j.Err,
		StartedAt:  formatTime(j.StartedAt),
		FinishedAt: formatTime(j.FinishedAt),
		Persisted:  j.Data.StoreID != "",
	}
	if rep := j.Data.Report; rep != nil {
		item.PagesCrawled = rep.PagesCrawled
		item.BySeverity = rep.Summary.BySeverity
	}
	item.StatusClasses, item.SitemapTotal, item.SitemapFetched, item.TapePreview = listProgress(j)
	return item
}
