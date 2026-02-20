package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codes/internal/agent"
)

// uniqueTeamName generates a unique team name for test isolation.
func uniqueTeamName(suffix string) string {
	return fmt.Sprintf("httptest-%s-%d", suffix, time.Now().UnixNano())
}

// --- Team CRUD ---

// TestCreateTeam tests POST /teams creates a new team.
func TestCreateTeam(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("create")

	body, _ := json.Marshal(CreateTeamRequest{
		Name:        teamName,
		Description: "test team",
	})

	req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	defer agent.DeleteTeam(teamName)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp TeamDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Name != teamName {
		t.Errorf("Expected team name %q, got %q", teamName, resp.Name)
	}
	if resp.Description != "test team" {
		t.Errorf("Expected description 'test team', got %q", resp.Description)
	}
}

// TestCreateTeamMissingName tests POST /teams without name field.
func TestCreateTeamMissingName(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(CreateTeamRequest{Description: "no name"})

	req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestCreateTeamDuplicate tests POST /teams with an existing team name returns 409.
func TestCreateTeamDuplicate(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("dup")

	// Create team first
	_, err := agent.CreateTeam(teamName, "first", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	// Try to create again via API
	body, _ := json.Marshal(CreateTeamRequest{Name: teamName})

	req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestTeamsMethodNotAllowed tests that PUT /teams returns 405.
func TestTeamsMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPut, "/teams", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestListTeams tests GET /teams.
func TestListTeams(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("list")

	_, err := agent.CreateTeam(teamName, "for listing", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	req := httptest.NewRequest(http.MethodGet, "/teams", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp TeamListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// The list should contain at least our test team
	found := false
	for _, team := range resp.Teams {
		if team.Name == teamName {
			found = true
			if team.Description != "for listing" {
				t.Errorf("Expected description 'for listing', got %q", team.Description)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected to find team %q in list", teamName)
	}
}

// TestGetTeam tests GET /teams/{name}.
func TestGetTeam(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("get")

	_, err := agent.CreateTeam(teamName, "detail test", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamName, nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp TeamDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Name != teamName {
		t.Errorf("Expected name %q, got %q", teamName, resp.Name)
	}
}

// TestGetTeamNotFound tests GET /teams/{name} for non-existent team.
func TestGetTeamNotFound(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/teams/nonexistent-team-xyz", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestDeleteTeam tests DELETE /teams/{name}.
func TestDeleteTeam(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("del")

	_, err := agent.CreateTeam(teamName, "to delete", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	// No defer cleanup needed since we're testing deletion

	req := httptest.NewRequest(http.MethodDelete, "/teams/"+teamName, nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify team is gone
	_, getErr := agent.GetTeam(teamName)
	if getErr == nil {
		t.Error("Expected team to be deleted, but GetTeam succeeded")
		agent.DeleteTeam(teamName) // cleanup if failed
	}
}

// TestDeleteTeamNotFound tests DELETE /teams/{nonexistent}.
func TestDeleteTeamNotFound(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodDelete, "/teams/nonexistent-team-xyz", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// --- Task CRUD ---

// TestCreateTeamTask tests POST /teams/{name}/tasks.
func TestCreateTeamTask(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("task")

	_, err := agent.CreateTeam(teamName, "task test", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	body, _ := json.Marshal(CreateTaskRequest{
		Subject:     "Test task",
		Description: "A task for testing",
		Priority:    "high",
	})

	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamName+"/tasks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Subject != "Test task" {
		t.Errorf("Expected subject 'Test task', got %q", resp.Subject)
	}
	if resp.Status != "pending" {
		t.Errorf("Expected status 'pending', got %q", resp.Status)
	}
	if resp.Priority != "high" {
		t.Errorf("Expected priority 'high', got %q", resp.Priority)
	}
}

// TestCreateTeamTaskMissingSubject tests POST /teams/{name}/tasks without subject.
func TestCreateTeamTaskMissingSubject(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("task-nosub")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	body, _ := json.Marshal(CreateTaskRequest{Description: "no subject"})

	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamName+"/tasks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestListTeamTasks tests GET /teams/{name}/tasks.
func TestListTeamTasks(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("tasklist")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	// Create two tasks directly
	agent.CreateTask(teamName, "Task 1", "desc 1", "", nil, agent.PriorityNormal, "", "")
	agent.CreateTask(teamName, "Task 2", "desc 2", "", nil, agent.PriorityHigh, "", "")

	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamName+"/tasks", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp TaskListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(resp.Tasks))
	}
}

// TestListTeamTasksMethodNotAllowed tests that DELETE /teams/{name}/tasks returns 405.
func TestListTeamTasksMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("taskmethod")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	req := httptest.NewRequest(http.MethodDelete, "/teams/"+teamName+"/tasks", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestUpdateTeamTask tests PATCH /teams/{name}/tasks/{id} with cancel action.
func TestUpdateTeamTask(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("taskupd")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	task, err := agent.CreateTask(teamName, "Cancellable task", "", "", nil, agent.PriorityNormal, "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	body, _ := json.Marshal(UpdateTaskRequest{Action: "cancel"})
	path := fmt.Sprintf("/teams/%s/tasks/%d", teamName, task.ID)

	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got %q", resp.Status)
	}
}

// TestUpdateTeamTaskAssign tests PATCH assign action.
func TestUpdateTeamTaskAssign(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("taskassign")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	task, err := agent.CreateTask(teamName, "Assign me", "", "", nil, agent.PriorityNormal, "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	body, _ := json.Marshal(UpdateTaskRequest{Action: "assign", Owner: "worker-1"})
	path := fmt.Sprintf("/teams/%s/tasks/%d", teamName, task.ID)

	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Owner != "worker-1" {
		t.Errorf("Expected owner 'worker-1', got %q", resp.Owner)
	}
}

// TestUpdateTeamTaskMissingAction tests PATCH without action field.
func TestUpdateTeamTaskMissingAction(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("tasknoactn")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	task, err := agent.CreateTask(teamName, "No action", "", "", nil, agent.PriorityNormal, "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"owner": "someone"})
	path := fmt.Sprintf("/teams/%s/tasks/%d", teamName, task.ID)

	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateTeamTaskComplete tests PATCH complete action.
func TestUpdateTeamTaskComplete(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("taskcomplete")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	task, err := agent.CreateTask(teamName, "Complete me", "", "", nil, agent.PriorityNormal, "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Must assign first — CompleteTask requires status to be "assigned" or "running"
	if _, err := agent.AssignTask(teamName, task.ID, "worker"); err != nil {
		t.Fatalf("Failed to assign task: %v", err)
	}

	body, _ := json.Marshal(UpdateTaskRequest{Action: "complete", Result: "all done"})
	path := fmt.Sprintf("/teams/%s/tasks/%d", teamName, task.ID)

	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "completed" {
		t.Errorf("Expected status 'completed', got %q", resp.Status)
	}
	if resp.Result != "all done" {
		t.Errorf("Expected result 'all done', got %q", resp.Result)
	}
}

// --- Messages ---

// TestSendTeamMessage tests POST /teams/{name}/messages.
func TestSendTeamMessage(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("msg")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	body, _ := json.Marshal(SendMessageRequest{
		From:    "user",
		To:      "worker",
		Content: "hello worker",
	})

	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamName+"/messages", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp MessageResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.From != "user" {
		t.Errorf("Expected from 'user', got %q", resp.From)
	}
	if resp.Content != "hello worker" {
		t.Errorf("Expected content 'hello worker', got %q", resp.Content)
	}
}

// TestSendTeamMessageMissingFields tests POST /teams/{name}/messages with missing fields.
func TestSendTeamMessageMissingFields(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("msgval")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	tests := []struct {
		name    string
		payload SendMessageRequest
	}{
		{"missing from", SendMessageRequest{Content: "hello"}},
		{"missing content", SendMessageRequest{From: "user"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/teams/"+teamName+"/messages", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d (body: %s)", w.Code, w.Body.String())
			}
		})
	}
}

// TestListTeamMessages tests GET /teams/{name}/messages.
func TestListTeamMessages(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("msglist")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	// Send a message directly
	agent.SendMessage(teamName, "alice", "bob", "hi bob")

	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamName+"/messages", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp MessageListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Messages) < 1 {
		t.Error("Expected at least 1 message")
	}
}

// --- Agent lifecycle ---

// TestStopTeamAgents tests POST /teams/{name}/stop with no running agents.
func TestStopTeamAgents(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("stop")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	// Add a member so there's something to stop
	agent.AddMember(teamName, agent.TeamMember{Name: "worker", Role: "test"})

	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamName+"/stop", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp StopTeamResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(resp.Results))
	}

	// Agent is not running, so it should be marked as stopped
	if !resp.Results[0].Stopped {
		t.Error("Expected agent to be reported as stopped")
	}
}

// TestStopTeamAgentsNotFound tests POST /teams/{nonexistent}/stop.
func TestStopTeamAgentsNotFound(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/teams/nonexistent-team-xyz/stop", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestStartTeamAgentsNotFound tests POST /teams/{nonexistent}/start.
func TestStartTeamAgentsNotFound(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/teams/nonexistent-team-xyz/start", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestStartTeamAgentsMethodNotAllowed tests that GET /teams/{name}/start returns 405.
func TestStartTeamAgentsMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("startmethod")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamName+"/start", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// --- Activity dashboard ---

// TestTeamActivity tests GET /teams/{name}/activity.
func TestTeamActivity(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("activity")

	_, err := agent.CreateTeam(teamName, "activity test", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	// Add a member and create a task for a non-trivial response
	agent.AddMember(teamName, agent.TeamMember{Name: "dev", Role: "developer"})
	agent.CreateTask(teamName, "Do something", "", "dev", nil, agent.PriorityNormal, "", "")

	req := httptest.NewRequest(http.MethodGet, "/teams/"+teamName+"/activity", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp TeamActivityResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Members) != 1 {
		t.Errorf("Expected 1 member, got %d", len(resp.Members))
	}
	if resp.Members[0].Name != "dev" {
		t.Errorf("Expected member name 'dev', got %q", resp.Members[0].Name)
	}
	if resp.TaskStats.Total != 1 {
		t.Errorf("Expected 1 total task, got %d", resp.TaskStats.Total)
	}
}

// TestTeamActivityNotFound tests GET /teams/{nonexistent}/activity.
func TestTeamActivityNotFound(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/teams/nonexistent-team-xyz/activity", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestTeamActivityMethodNotAllowed tests that POST /teams/{name}/activity returns 405.
func TestTeamActivityMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")
	teamName := uniqueTeamName("actmethod")

	_, err := agent.CreateTeam(teamName, "", "")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer agent.DeleteTeam(teamName)

	req := httptest.NewRequest(http.MethodPost, "/teams/"+teamName+"/activity", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}
