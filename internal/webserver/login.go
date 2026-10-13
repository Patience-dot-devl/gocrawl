package webserver

// loginManager tracks interactive browser-login sessions. The browser-login feature fills
// in its fields.
type loginManager struct{}

func newLoginManager() *loginManager {
	return &loginManager{}
}

// loginRoutes registers the /api/login-sessions routes. Empty until the browser-login
// feature fills it in.
func (s *Server) loginRoutes() {}
