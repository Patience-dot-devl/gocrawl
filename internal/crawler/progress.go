package crawler

// Progress is a snapshot of a running crawl, delivered through Options.OnProgress after each
// page is recorded (and, from the pause track on, on every pause and resume).
type Progress struct {
	// Fetched is the number of pages recorded so far, including this one.
	Fetched int `json:"fetched"`
	// Queued is the number of URLs waiting in the frontier.
	Queued int `json:"queued"`
	// CurrentURL is the URL just recorded, as requested; FinalURL is where it landed after
	// redirects.
	CurrentURL string `json:"current_url"`
	FinalURL   string `json:"final_url,omitempty"`
	// Status is the HTTP status of the page just recorded: the first hop's status when the
	// fetch was redirected (so a redirect reads as 3xx), otherwise the final status, and 0
	// when the fetch failed.
	Status int `json:"status"`
	// DurationMS is how long that fetch took.
	DurationMS int64 `json:"duration_ms"`
	// Rate is the requests-per-second limit in effect (0 = unlimited), after any backoff.
	Rate float64 `json:"rate"`
	// Throttled counts HTTP 429/503 responses in the last 60 seconds.
	Throttled int `json:"throttled"`
	// Paused, PauseReason ("user" or "rate_limited") and RetryAfterSeconds describe a pause.
	Paused            bool   `json:"paused"`
	PauseReason       string `json:"pause_reason,omitempty"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
}

// progressStatus is the status a page reports in Progress.Status.
func progressStatus(p *Page) int {
	if len(p.Redirects) > 0 {
		return p.Redirects[0].Status
	}
	return p.StatusCode
}

// emitPage delivers a page event. The caller holds the crawl's result mutex, so page events
// reach OnProgress in recording order; progMu serializes them with state-only events.
func (e *Engine) emitPage(p Progress) {
	if e.opts.OnProgress == nil {
		return
	}
	e.progMu.Lock()
	defer e.progMu.Unlock()
	e.fillLive(&p)
	e.lastProgress = p
	e.opts.OnProgress(p)
}

// fillLive copies the fields that change between pages: the rate limit and recent throttling.
func (e *Engine) fillLive(p *Progress) {
	p.Rate = e.limiter.EffectiveRate()
	p.Throttled = e.limiter.RecentThrottled()
}
