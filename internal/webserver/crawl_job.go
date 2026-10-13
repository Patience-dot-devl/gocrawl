package webserver

import (
	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// crawlJob is the crawl-specific part of a Job: what the web API knows about one crawl
// beyond its lifecycle. Later features add their fields here (live progress and the tape,
// the pause control).
type crawlJob struct {
	Seed    string
	Report  *report.Report
	StoreID string // set once the report is saved, so history and job views agree on one ID
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
	return item
}
