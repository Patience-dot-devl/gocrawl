package webserver

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
)

// multipartMemory is how much of a multipart body ParseMultipartForm keeps in memory; larger
// file parts spill to temporary files, which the caller removes with
// r.MultipartForm.RemoveAll.
const multipartMemory = 8 << 20

// requireJSON writes 415 and returns false unless the request body is declared as JSON. A
// cross-origin HTML form can only send text/plain or form encodings without a CORS
// preflight, so insisting on JSON is the second half of the CSRF guard in guard.go. Every
// JSON-bodied POST, PUT, and DELETE handler calls it first.
func requireJSON(w http.ResponseWriter, r *http.Request) bool {
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, errors.New("Content-Type must be application/json"))
		return false
	}
	return true
}

// parseMultipart parses a multipart/form-data upload with the body capped at maxBytes. On
// failure it writes the error response (415 for another content type, 413 over the cap, 400
// for a malformed body) and returns false. On success r.MultipartForm is populated; the
// caller must `defer r.MultipartForm.RemoveAll()` to delete parts spilled to disk. The
// origin check for these routes is the guard's, like every other non-GET route.
func parseMultipart(w http.ResponseWriter, r *http.Request, maxBytes int64) bool {
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "multipart/form-data" {
		writeError(w, http.StatusUnsupportedMediaType, errors.New("Content-Type must be multipart/form-data"))
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(min(maxBytes, multipartMemory)); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("upload is larger than the %s limit", byteSize(maxBytes)))
			return false
		}
		writeError(w, http.StatusBadRequest, fmt.Errorf("reading multipart form: %w", err))
		return false
	}
	return true
}

// byteSize renders an upload limit for an error message: whole megabytes as "10 MB",
// anything else in bytes.
func byteSize(n int64) string {
	if n >= 1<<20 && n%(1<<20) == 0 {
		return fmt.Sprintf("%d MB", n>>20)
	}
	return fmt.Sprintf("%d bytes", n)
}
