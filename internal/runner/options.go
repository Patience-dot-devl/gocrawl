package runner

// RunOption customizes one Run call. The CLI and MCP server call Run without any; the web
// server uses options for what only it needs (live progress, pause and resume), so Run's
// signature stays the same for every caller.
type RunOption func(*runSettings)

// runSettings collects the RunOption values for one Run call. Each feature adds its own
// field here together with the RunOption constructor that sets it, never a new positional
// parameter on Run.
type runSettings struct{}

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
