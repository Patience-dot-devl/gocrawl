package webserver

// WithGUISettingsPath sets the file GET and PUT /api/settings read and write (gocrawl serve
// passes ~/.gocrawl/gui.json). An empty path, the default, means settings are not stored.
func WithGUISettingsPath(path string) Option {
	return func(s *Server) { s.guiSettingsPath = path }
}

// settingsRoutes registers GET and PUT /api/settings. Empty until the settings feature fills
// it in.
func (s *Server) settingsRoutes() {}
