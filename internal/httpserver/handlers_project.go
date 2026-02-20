package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"codes/internal/config"
)

// handleListProjects handles GET /projects
func (s *HTTPServer) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	projects, err := config.ListProjects()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list projects: %v", err))
		return
	}

	list := make([]ProjectInfoResponse, 0, len(projects))
	for name, entry := range projects {
		list = append(list, ProjectInfoResponse{
			Name: name,
			Path: entry.Path,
			Host: entry.Remote,
		})
	}

	respondJSON(w, http.StatusOK, ProjectListResponse{Projects: list})
}

// handleGetProject handles GET /projects/{name}
func (s *HTTPServer) handleGetProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse path: /projects/{name}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /projects/{name})")
		return
	}

	name := parts[1]
	if name == "" {
		respondError(w, http.StatusBadRequest, "project name is required")
		return
	}

	entry, exists := config.GetProject(name)
	if !exists {
		respondError(w, http.StatusNotFound, fmt.Sprintf("project %q not found", name))
		return
	}

	respondJSON(w, http.StatusOK, ProjectInfoResponse{
		Name: name,
		Path: entry.Path,
		Host: entry.Remote,
	})
}

// handleListProfiles handles GET /profiles
func (s *HTTPServer) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load config: %v", err))
		return
	}

	profiles := make([]ProfileInfo, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		profiles = append(profiles, ProfileInfo{
			Name:      p.Name,
			IsDefault: p.Name == cfg.Default,
		})
	}

	respondJSON(w, http.StatusOK, ProfileListResponse{Profiles: profiles})
}

// handleSwitchProfile handles POST /profiles/switch
func (s *HTTPServer) handleSwitchProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req SwitchProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "field 'name' is required")
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load config: %v", err))
		return
	}

	// Verify the profile exists
	found := false
	for _, p := range cfg.Profiles {
		if p.Name == req.Name {
			found = true
			break
		}
	}
	if !found {
		respondError(w, http.StatusNotFound, fmt.Sprintf("profile %q not found", req.Name))
		return
	}

	cfg.Default = req.Name
	if err := config.SaveConfig(cfg); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, SwitchProfileResponse{
		Message: fmt.Sprintf("switched to profile %q", req.Name),
		Active:  req.Name,
	})
}
