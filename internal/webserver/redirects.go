package webserver

// redirectJob is the redirect-check-specific part of a Job, run by its own jobManager with
// its own concurrency cap. The redirect-check feature fills in its fields.
type redirectJob struct{}

// redirectRoutes registers the /api/redirect-checks routes. Empty until the redirect-check
// feature fills it in.
func (s *Server) redirectRoutes() {}
