package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"codes/internal/agent"
)

// TestGetTaskByPath tests GET /tasks/{team}/{id} returns task details.
func TestGetTaskByPath(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("gettask")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	task, err := agent.CreateTask(teamName, "My task", "task desc", "", nil, agent.PriorityNormal, "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	path := fmt.Sprintf("/tasks/%s/%d", teamName, task.ID)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.ID != task.ID {
		t.Errorf("Expected task ID %d, got %d", task.ID, resp.ID)
	}
	if resp.Subject != "My task" {
		t.Errorf("Expected subject 'My task', got %q", resp.Subject)
	}
	if resp.Status != "pending" {
		t.Errorf("Expected status 'pending', got %q", resp.Status)
	}
}

// TestGetTaskByPathNotFound tests GET /tasks/{team}/99999 returns 404.
func TestGetTaskByPathNotFound(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("gettask-nf")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/tasks/%s/99999", teamName), nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestGetTaskByPathInvalidID tests GET /tasks/{team}/abc returns 400.
func TestGetTaskByPathInvalidID(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/tasks/someteam/abc", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestGetTaskByPathInvalidPath tests GET /tasks/{team} (missing id segment) returns 400.
func TestGetTaskByPathInvalidPath(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/tasks/onlyteam/", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestGetTaskByPathMethodNotAllowed tests POST /tasks/{team}/{id} returns 405.
func TestGetTaskByPathMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/tasks/someteam/1", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestGetTaskByPathRequiresAuth tests that /tasks/{team}/{id} requires authentication.
func TestGetTaskByPathRequiresAuth(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/tasks/someteam/1", nil)
	// No Authorization header

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}
