package runner

// RunOption customizes one Run call. The CLI and MCP server call Run without any; the web
// server uses options for what only it needs (live progress, pause and resume), so Run's
// signature stays the same for every caller.
type RunOption func(*runSettings)

// runSettings collects the RunOption values for one Run call. Each feature adds its own
// field here together with the RunOption constructor that sets it, never a new positional
// parameter on Run.
type runSettings struct {
	// progress receives a Progress snapshot per recorded page and per phase change.
	progress func(Progress)
	// control forwards Pause/Resume to the engine during the crawl phase.
	control *Control
	// pauseOnThrottle sets crawler.Options.PauseOnThrottle.
	pauseOnThrottle bool
}

// WithProgress makes Run report progress to fn: one call per phase change, one per page the
// crawl records, and one per pause or resume, serialized and in order. fn runs on crawl
// goroutines (or, for a pause or resume, on the goroutine that asked for it) while the engine
// holds its locks, so it must return quickly and must not call a Control method.
func WithProgress(fn func(Progress)) RunOption {
	return func(s *runSettings) { s.progress = fn }
}

// WithControl lets the caller pause and resume this Run's crawl through c.
func WithControl(c *Control) RunOption {
	return func(s *runSettings) { s.control = c }
}

// WithPauseOnThrottle makes the crawl pause itself after sustained 429/503 responses (see
// crawler.Options.PauseOnThrottle). Only for callers that can resume it, like the web UI.
func WithPauseOnThrottle() RunOption {
	return func(s *runSettings) { s.pauseOnThrottle = true }
}

// newRunSettings applies opts in order, skipping nil entries so a caller can pass an
// optional option without a branch.
func newRunSettings(opts []RunOption) runSettings {
	var rs runSettings
	for _, o := range opts {
		if o != nil {
			o(&rs)
		}
	}
	return rs
}
