package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"codes/internal/agent"
	"codes/internal/dispatch"
)

// handleHealth handles GET /health
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	respondJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Version: s.version,
	})
}

// handleDispatch handles POST /dispatch using smart intent analysis.
func (s *HTTPServer) handleDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req DispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Text == "" {
		respondError(w, http.StatusBadRequest, "field 'text' is required")
		return
	}

	if req.Channel == "" {
		req.Channel = "http"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	result, err := dispatch.Dispatch(ctx, dispatch.DispatchOptions{
		UserInput:   req.Text,
		Channel:     req.Channel,
		ChatID:      req.ChatID,
		Project:     req.Project,
		CallbackURL: req.CallbackURL,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("dispatch failed: %v", err))
		return
	}

	if result.Clarify != "" {
		respondJSON(w, http.StatusOK, SmartDispatchResponse{
			Clarify:  result.Clarify,
			Duration: result.DurationStr,
		})
		return
	}

	if result.Error != "" {
		respondError(w, http.StatusBadRequest, result.Error)
		return
	}

	respondJSON(w, http.StatusCreated, SmartDispatchResponse{
		Team:          result.TeamName,
		TasksCreated:  result.TasksCreated,
		AgentsStarted: result.AgentsStarted,
		Duration:      result.DurationStr,
	})
}

// handleDispatchSimple handles POST /dispatch/simple (legacy single-worker dispatch).
func (s *HTTPServer) handleDispatchSimple(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse request body (limit to 1MB)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req DispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	// Validate required fields
	if req.Text == "" {
		respondError(w, http.StatusBadRequest, "field 'text' is required")
		return
	}

	// Default values
	if req.Channel == "" {
		req.Channel = "http"
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}

	// Generate team name with nanosecond timestamp + random suffix to avoid collisions
	randBuf := make([]byte, 2)
	_, _ = rand.Read(randBuf)
	randSuffix := int(randBuf[0])<<8 | int(randBuf[1])
	teamName := fmt.Sprintf("dispatch-%s-%d-%04d", req.Channel, time.Now().UnixNano(), randSuffix%10000)

	// Create team
	teamDesc := fmt.Sprintf("Dispatch from %s", req.Channel)
	if req.ChatID != "" {
		teamDesc = fmt.Sprintf("Dispatch from %s (chat: %s)", req.Channel, req.ChatID)
	}

	if _, err := agent.CreateTeam(teamName, teamDesc, ""); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create team: %v", err))
		return
	}

	// Cleanup team on failure
	success := false
	defer func() {
		if !success {
			agent.DeleteTeam(teamName)
		}
	}()

	// Add worker agent
	workerName := "worker"
	member := agent.TeamMember{
		Name:  workerName,
		Role:  "Execute dispatched tasks",
		Model: "sonnet",
		Type:  "worker",
	}
	if err := agent.AddMember(teamName, member); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to add worker: %v", err))
		return
	}

	// Create task
	taskSubject := fmt.Sprintf("Request from %s", req.Channel)
	taskDesc := req.Text

	// Parse priority
	var priority agent.TaskPriority
	switch req.Priority {
	case "high":
		priority = agent.PriorityHigh
	case "low":
		priority = agent.PriorityLow
	default:
		priority = agent.PriorityNormal
	}

	task, err := agent.CreateTask(teamName, taskSubject, taskDesc, workerName, nil, priority, req.Project, "")
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create task: %v", err))
		return
	}

	// Store callback URL if provided
	if req.CallbackURL != "" {
		task, err = agent.UpdateTask(teamName, task.ID, func(t *agent.Task) error {
			t.CallbackURL = req.CallbackURL
			return nil
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to store callback URL: %v", err))
			return
		}
	}

	// Start agent
	if _, err := agent.StartAgent(teamName, workerName); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start agent: %v", err))
		return
	}

	// Return response
	success = true
	respondJSON(w, http.StatusCreated, DispatchResponse{
		TaskID: task.ID,
		Team:   teamName,
		Status: string(task.Status),
	})
}

// handleGetTask handles GET /tasks/{team}/{id}
func (s *HTTPServer) handleGetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse path: /tasks/{team}/{id}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /tasks/{team}/{id})")
		return
	}

	teamName := parts[1]
	taskIDStr := parts[2]

	if teamName == "" {
		respondError(w, http.StatusBadRequest, "team name is required")
		return
	}
	if taskIDStr == "" {
		respondError(w, http.StatusBadRequest, "task ID is required")
		return
	}

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid task ID")
		return
	}

	// Get task
	task, err := agent.GetTask(teamName, taskID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("task not found: %v", err))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get task: %v", err))
		return
	}

	// Convert to response
	resp := TaskResponse{
		ID:          task.ID,
		Subject:     task.Subject,
		Description: task.Description,
		Status:      string(task.Status),
		Priority:    string(task.Priority),
		Owner:       task.Owner,
		Project:     task.Project,
		WorkDir:     task.WorkDir,
		Result:      task.Result,
		Error:       task.Error,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
		CompletedAt: task.CompletedAt,
	}

	respondJSON(w, http.StatusOK, resp)
}

// handleListTeams handles GET /teams
func (s *HTTPServer) handleListTeams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	teamNames, err := agent.ListTeams()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list teams: %v", err))
		return
	}

	// Get details for each team
	summaries := make([]TeamSummary, 0, len(teamNames))
	for _, name := range teamNames {
		team, err := agent.GetTeam(name)
		if err != nil {
			continue // Skip teams we can't read
		}

		summaries = append(summaries, TeamSummary{
			Name:        team.Name,
			Description: team.Description,
			MemberCount: len(team.Members),
			CreatedAt:   team.CreatedAt,
		})
	}

	respondJSON(w, http.StatusOK, TeamListResponse{Teams: summaries})
}

// handleGetTeam handles GET /teams/{name}
func (s *HTTPServer) handleGetTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse path: /teams/{name}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /teams/{name})")
		return
	}

	teamName := parts[1]

	if teamName == "" {
		respondError(w, http.StatusBadRequest, "team name is required")
		return
	}

	// Get team config
	team, err := agent.GetTeam(teamName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("team not found: %v", err))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get team: %v", err))
		return
	}

	// Get agent states for status information
	members := make([]TeamMember, 0, len(team.Members))
	for _, m := range team.Members {
		member := TeamMember{
			Name:  m.Name,
			Role:  m.Role,
			Model: m.Model,
			Type:  m.Type,
		}

		// Try to get agent state
		state, err := agent.GetAgentState(teamName, m.Name)
		if err == nil && state != nil {
			member.Status = string(state.Status)
			member.PID = state.PID
		} else {
			member.Status = "stopped"
		}

		members = append(members, member)
	}

	// Convert to response
	resp := TeamDetailResponse{
		Name:        team.Name,
		Description: team.Description,
		WorkDir:     team.WorkDir,
		Members:     members,
		CreatedAt:   team.CreatedAt,
	}

	respondJSON(w, http.StatusOK, resp)
}
