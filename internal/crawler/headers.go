package crawler

import (
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/http/httpguts"
)

// reservedHeaders are header names ParseHeaderLines refuses, each with the reason shown to the
// user. They are either owned by net/http's transport (setting them by hand breaks framing,
// connection reuse, or transparent gzip decoding), or already have a dedicated option whose
// behavior a raw header would silently bypass.
var reservedHeaders = map[string]string{
	"Host":                "set by the HTTP client from the request URL",
	"Content-Length":      "managed by the HTTP client",
	"Transfer-Encoding":   "managed by the HTTP client",
	"Connection":          "a hop-by-hop header managed by the HTTP client",
	"Keep-Alive":          "a hop-by-hop header managed by the HTTP client",
	"Proxy-Connection":    "a hop-by-hop header managed by the HTTP client",
	"Proxy-Authorization": "a hop-by-hop header; put proxy credentials in the proxy URL instead",
	"Te":                  "a hop-by-hop header managed by the HTTP client",
	"Trailer":             "a hop-by-hop header managed by the HTTP client",
	"Upgrade":             "a hop-by-hop header managed by the HTTP client",
	"Accept-Encoding":     "managed by the HTTP client, which only decompresses responses when it sets this itself",
	"Cookie":              "use --cookie / cookie instead",
	"User-Agent":          "use --user-agent / user_agent (or --user-agents) instead",
}

// ParseHeaderLines parses "Name: value" lines into an http.Header. Blank lines and lines
// starting with '#' are skipped, so the same parser reads both --header flags and a header
// file. A name given twice keeps the later value — so an inline --header can override one
// from a file — rather than sending the header twice. Names and values are validated as HTTP
// field syntax (which rules out CR/LF header injection), and names net/http manages itself or
// that have a dedicated option are refused; see reservedHeaders.
func ParseHeaderLines(lines []string) (http.Header, error) {
	h := http.Header{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf(`header %q: want "Name: value"`, line)
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if !httpguts.ValidHeaderFieldName(name) {
			return nil, fmt.Errorf("header name %q is not a valid HTTP field name", name)
		}
		if !httpguts.ValidHeaderFieldValue(value) {
			return nil, fmt.Errorf("header %s: value contains characters not allowed in an HTTP header", name)
		}
		canonical := http.CanonicalHeaderKey(name)
		if why, reserved := reservedHeaders[canonical]; reserved {
			return nil, fmt.Errorf("header %s can't be set as a custom header: %s", canonical, why)
		}
		h.Set(canonical, value)
	}
	return h, nil
}
