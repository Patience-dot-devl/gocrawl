package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
)

// headerProfileView is GET /api/header-profiles/{name}. The body holds secrets; it is only
// served to the loopback, same-origin UI the guard admits, so the editor can show it.
type headerProfileView struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

// maxProfileBody caps a PUT request body, so a profile can't grow without bound.
const maxProfileBody = 64 << 10

// profileRoutes registers the /api/header-profiles routes.
func (s *Server) profileRoutes() {
	s.mux.HandleFunc("GET /api/header-profiles", s.handleListProfiles)
	s.mux.HandleFunc("GET /api/header-profiles/{name}", s.handleGetProfile)
	s.mux.HandleFunc("PUT /api/header-profiles/{name}", s.handlePutProfile)
	s.mux.HandleFunc("DELETE /api/header-profiles/{name}", s.handleDeleteProfile)
}

func (s *Server) handleListProfiles(w http.ResponseWriter, _ *http.Request) {
	names, err := config.ListHeaderProfiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": names})
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	body, err := config.ReadHeaderProfile(name)
	if err != nil {
		writeError(w, profileErrorStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, headerProfileView{Name: name, Body: body})
}

func (s *Server) handlePutProfile(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxProfileBody)).Decode(&in); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("a header profile can't be larger than %d bytes", maxProfileBody))
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Errorf("decoding request body: %w", err))
		return
	}
	if err := config.SaveHeaderProfile(r.PathValue("name"), in.Body); err != nil {
		writeError(w, profileErrorStatus(err), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	if err := config.DeleteHeaderProfile(r.PathValue("name")); err != nil {
		writeError(w, profileErrorStatus(err), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// profileErrorStatus maps a config header-profile error to its HTTP status.
func profileErrorStatus(err error) int {
	switch {
	case errors.Is(err, config.ErrInvalidHeaderProfile):
		return http.StatusBadRequest
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
