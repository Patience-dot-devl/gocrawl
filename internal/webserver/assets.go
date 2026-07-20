package webserver

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// embeddedWebUI holds the built single-page app (internal/webserver/webui/dist). A minimal
// placeholder index.html ships in git so a bare `go build`/`go vet`/`go test` always succeeds
// with no Node dependency; `make web-build` overwrites it with the real UI.
//
//go:embed all:webui/dist
var embeddedWebUI embed.FS

// frontendHandler serves the embedded SPA, falling back to index.html for any path that isn't
// a real asset so client-side routing works on a hard refresh or deep link.
func frontendHandler() http.Handler {
	sub, err := fs.Sub(embeddedWebUI, "webui/dist")
	if err != nil {
		panic(err) // the embed directive guarantees webui/dist exists at compile time
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "."
		}
		if _, statErr := fs.Stat(sub, name); statErr != nil {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/"
			fileServer.ServeHTTP(w, clone)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
