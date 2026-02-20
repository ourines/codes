package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"codes/internal/workflow"
)

// handleListWorkflows handles GET /workflows
func (s *HTTPServer) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	workflows, err := workflow.ListWorkflows()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list workflows: %v", err))
		return
	}

	list := make([]WorkflowSummary, 0, len(workflows))
	for _, wf := range workflows {
		list = append(list, WorkflowSummary{
			Name:        wf.Name,
			Description: wf.Description,
			AgentCount:  len(wf.Agents),
			TaskCount:   len(wf.Tasks),
			BuiltIn:     wf.BuiltIn,
		})
	}

	respondJSON(w, http.StatusOK, WorkflowListResponse{Workflows: list})
}

// handleGetWorkflow handles GET /workflows/{name}
func (s *HTTPServer) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse path: /workflows/{name}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /workflows/{name})")
		return
	}

	name := parts[1]
	if name == "" {
		respondError(w, http.StatusBadRequest, "workflow name is required")
		return
	}

	wf, err := workflow.GetWorkflow(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("workflow %q not found", name))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get workflow: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, wf)
}

// handleRunWorkflow handles POST /workflows/{name}/run
func (s *HTTPServer) handleRunWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse path: /workflows/{name}/run
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "run" {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /workflows/{name}/run)")
		return
	}

	name := parts[1]
	if name == "" {
		respondError(w, http.StatusBadRequest, "workflow name is required")
		return
	}

	// Parse optional request body
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req RunWorkflowRequest
	// Body is optional; ignore decode errors for empty body
	_ = json.NewDecoder(r.Body).Decode(&req)

	wf, err := workflow.GetWorkflow(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("workflow %q not found", name))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get workflow: %v", err))
		return
	}

	opts := workflow.RunWorkflowOptions{
		WorkDir: req.WorkDir,
		Model:   req.Model,
		Project: req.Project,
	}

	result, err := workflow.RunWorkflow(wf, opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to run workflow: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, RunWorkflowResponse{
		TeamName:      result.TeamName,
		AgentsStarted: result.Agents,
		TasksCreated:  result.Tasks,
	})
}
