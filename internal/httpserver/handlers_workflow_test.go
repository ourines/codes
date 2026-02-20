package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codes/internal/workflow"
)

// TestListWorkflows tests GET /workflows returns built-in workflows.
func TestListWorkflows(t *testing.T) {
	// Ensure built-in workflows are written to disk
	if err := workflow.EnsureBuiltins(); err != nil {
		t.Fatalf("Failed to ensure builtins: %v", err)
	}

	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/workflows", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp WorkflowListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Workflows) == 0 {
		t.Fatal("Expected at least 1 workflow (built-ins), got 0")
	}

	// Check that built-in "code-review" is in the list
	found := false
	for _, wf := range resp.Workflows {
		if wf.Name == "code-review" {
			found = true
			if !wf.BuiltIn {
				t.Error("Expected code-review to be marked as built-in")
			}
			break
		}
	}
	if !found {
		t.Error("Expected to find 'code-review' in workflow list")
	}
}

// TestListWorkflowsMethodNotAllowed tests that POST /workflows returns 405.
func TestListWorkflowsMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/workflows", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestGetWorkflow tests GET /workflows/{name} for a built-in workflow.
func TestGetWorkflow(t *testing.T) {
	if err := workflow.EnsureBuiltins(); err != nil {
		t.Fatalf("Failed to ensure builtins: %v", err)
	}

	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/workflows/code-review", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify it returns valid JSON with expected fields
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["name"] != "code-review" {
		t.Errorf("Expected name 'code-review', got %v", resp["name"])
	}
}

// TestGetWorkflowNotFound tests GET /workflows/{name} for non-existent workflow.
func TestGetWorkflowNotFound(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/workflows/nonexistent-wf-xyz", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestGetWorkflowMethodNotAllowed tests that POST /workflows/{name} returns 405.
func TestGetWorkflowMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/workflows/code-review", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestRunWorkflowNotFound tests POST /workflows/{nonexistent}/run.
func TestRunWorkflowNotFound(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(RunWorkflowRequest{})
	req := httptest.NewRequest(http.MethodPost, "/workflows/nonexistent-wf-xyz/run", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestRunWorkflowMethodNotAllowed tests that GET /workflows/{name}/run returns 405.
func TestRunWorkflowMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/workflows/code-review/run", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	// GET on /workflows/code-review/run should hit the routeWorkflow handler
	// which checks parts[2] == "run" but requires POST
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d (body: %s)", w.Code, w.Body.String())
	}
}
