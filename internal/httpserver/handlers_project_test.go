package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"codes/internal/config"
)

// setupTestConfig creates a temp config file and overrides config.ConfigPath.
// Returns a cleanup function that restores the original path.
func setupTestConfig(t *testing.T, cfg *config.Config) func() {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	origPath := config.ConfigPath
	config.ConfigPath = configPath

	return func() {
		config.ConfigPath = origPath
	}
}

// TestListProjects tests GET /projects returns project list.
func TestListProjects(t *testing.T) {
	cleanup := setupTestConfig(t, &config.Config{
		Profiles: []config.APIConfig{{Name: "default"}},
		Default:  "default",
		Projects: map[string]config.ProjectEntry{
			"my-app":    {Path: "/home/user/my-app"},
			"other-app": {Path: "/home/user/other-app", Remote: "remote-host"},
		},
	})
	defer cleanup()

	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp ProjectListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(resp.Projects))
	}

	// Verify project data is present (order is not guaranteed for maps)
	found := make(map[string]bool)
	for _, p := range resp.Projects {
		found[p.Name] = true
	}
	if !found["my-app"] || !found["other-app"] {
		t.Errorf("Missing expected projects in response: %v", resp.Projects)
	}
}

// TestListProjectsEmpty tests GET /projects with no configured projects.
func TestListProjectsEmpty(t *testing.T) {
	cleanup := setupTestConfig(t, &config.Config{
		Profiles: []config.APIConfig{{Name: "default"}},
		Default:  "default",
	})
	defer cleanup()

	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp ProjectListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(resp.Projects))
	}
}

// TestListProjectsMethodNotAllowed tests that POST /projects returns 405.
func TestListProjectsMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/projects", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestGetProject tests GET /projects/{name} returns project details.
func TestGetProject(t *testing.T) {
	cleanup := setupTestConfig(t, &config.Config{
		Profiles: []config.APIConfig{{Name: "default"}},
		Default:  "default",
		Projects: map[string]config.ProjectEntry{
			"my-app": {Path: "/home/user/my-app"},
		},
	})
	defer cleanup()

	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/projects/my-app", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp ProjectInfoResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Name != "my-app" {
		t.Errorf("Expected name 'my-app', got %q", resp.Name)
	}
	if resp.Path != "/home/user/my-app" {
		t.Errorf("Expected path '/home/user/my-app', got %q", resp.Path)
	}
}

// TestGetProjectNotFound tests GET /projects/{name} for a non-existent project.
func TestGetProjectNotFound(t *testing.T) {
	cleanup := setupTestConfig(t, &config.Config{
		Profiles: []config.APIConfig{{Name: "default"}},
		Default:  "default",
		Projects: map[string]config.ProjectEntry{},
	})
	defer cleanup()

	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/projects/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestListProfiles tests GET /profiles returns profile list.
func TestListProfiles(t *testing.T) {
	cleanup := setupTestConfig(t, &config.Config{
		Profiles: []config.APIConfig{
			{Name: "default"},
			{Name: "staging"},
		},
		Default: "default",
	})
	defer cleanup()

	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/profiles", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp ProfileListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Profiles) != 2 {
		t.Fatalf("Expected 2 profiles, got %d", len(resp.Profiles))
	}

	// Check that default profile is marked
	for _, p := range resp.Profiles {
		if p.Name == "default" && !p.IsDefault {
			t.Error("Expected default profile to have is_default=true")
		}
		if p.Name == "staging" && p.IsDefault {
			t.Error("Expected staging profile to have is_default=false")
		}
	}
}

// TestListProfilesMethodNotAllowed tests that POST /profiles returns 405.
func TestListProfilesMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/profiles", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestSwitchProfile tests POST /profiles/switch.
func TestSwitchProfile(t *testing.T) {
	cleanup := setupTestConfig(t, &config.Config{
		Profiles: []config.APIConfig{
			{Name: "default"},
			{Name: "staging"},
		},
		Default: "default",
	})
	defer cleanup()

	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(SwitchProfileRequest{Name: "staging"})
	req := httptest.NewRequest(http.MethodPost, "/profiles/switch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp SwitchProfileResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Active != "staging" {
		t.Errorf("Expected active profile 'staging', got %q", resp.Active)
	}
}

// TestSwitchProfileNotFound tests POST /profiles/switch with unknown profile.
func TestSwitchProfileNotFound(t *testing.T) {
	cleanup := setupTestConfig(t, &config.Config{
		Profiles: []config.APIConfig{{Name: "default"}},
		Default:  "default",
	})
	defer cleanup()

	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(SwitchProfileRequest{Name: "nonexistent"})
	req := httptest.NewRequest(http.MethodPost, "/profiles/switch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestSwitchProfileMissingName tests POST /profiles/switch without name field.
func TestSwitchProfileMissingName(t *testing.T) {
	cleanup := setupTestConfig(t, &config.Config{
		Profiles: []config.APIConfig{{Name: "default"}},
		Default:  "default",
	})
	defer cleanup()

	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/profiles/switch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestSwitchProfileMethodNotAllowed tests that GET /profiles/switch returns 405.
func TestSwitchProfileMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/profiles/switch", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}
