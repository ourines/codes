package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"codes/internal/agent"
)

// --- Conversion helpers ---

func taskToResponse(t *agent.Task) TaskResponse {
	return TaskResponse{
		ID:          t.ID,
		Subject:     t.Subject,
		Description: t.Description,
		Status:      string(t.Status),
		Priority:    string(t.Priority),
		Owner:       t.Owner,
		Project:     t.Project,
		WorkDir:     t.WorkDir,
		Result:      t.Result,
		Error:       t.Error,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		CompletedAt: t.CompletedAt,
	}
}

func messageToResponse(m *agent.Message) MessageResponse {
	return MessageResponse{
		ID:        m.ID,
		Type:      string(m.Type),
		From:      m.From,
		To:        m.To,
		Content:   m.Content,
		TaskID:    m.TaskID,
		Read:      m.Read,
		CreatedAt: m.CreatedAt,
	}
}

// --- Team management handlers ---

// handleCreateTeam handles POST /teams
func (s *HTTPServer) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "field 'name' is required")
		return
	}

	team, err := agent.CreateTeam(req.Name, req.Description, req.WorkDir)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			respondError(w, http.StatusConflict, fmt.Sprintf("team already exists: %v", err))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create team: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, TeamDetailResponse{
		Name:        team.Name,
		Description: team.Description,
		WorkDir:     team.WorkDir,
		Members:     []TeamMember{},
		CreatedAt:   team.CreatedAt,
	})
}

// handleDeleteTeam handles DELETE /teams/{name}
func (s *HTTPServer) handleDeleteTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

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

	if err := agent.DeleteTeam(teamName); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("team not found: %v", err))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete team: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "team deleted"})
}

// --- Task handlers ---

// handleListTeamTasks handles GET /teams/{name}/tasks
func (s *HTTPServer) handleListTeamTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "tasks" {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /teams/{name}/tasks)")
		return
	}

	teamName := parts[1]
	if teamName == "" {
		respondError(w, http.StatusBadRequest, "team name is required")
		return
	}

	// Parse query filters
	statusFilter := agent.TaskStatus(r.URL.Query().Get("status"))
	ownerFilter := r.URL.Query().Get("owner")

	tasks, err := agent.ListTasks(teamName, statusFilter, ownerFilter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list tasks: %v", err))
		return
	}

	resp := make([]TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		resp = append(resp, taskToResponse(t))
	}

	respondJSON(w, http.StatusOK, TaskListResponse{Tasks: resp})
}

// handleCreateTeamTask handles POST /teams/{name}/tasks
func (s *HTTPServer) handleCreateTeamTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "tasks" {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /teams/{name}/tasks)")
		return
	}

	teamName := parts[1]
	if teamName == "" {
		respondError(w, http.StatusBadRequest, "team name is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Subject == "" {
		respondError(w, http.StatusBadRequest, "field 'subject' is required")
		return
	}

	var priority agent.TaskPriority
	switch req.Priority {
	case "high":
		priority = agent.PriorityHigh
	case "low":
		priority = agent.PriorityLow
	default:
		priority = agent.PriorityNormal
	}

	task, err := agent.CreateTask(teamName, req.Subject, req.Description, req.Owner, req.BlockedBy, priority, req.Project, req.WorkDir)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create task: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, taskToResponse(task))
}

// handleUpdateTeamTask handles PATCH /teams/{name}/tasks/{id}
func (s *HTTPServer) handleUpdateTeamTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "tasks" {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /teams/{name}/tasks/{id})")
		return
	}

	teamName := parts[1]
	if teamName == "" {
		respondError(w, http.StatusBadRequest, "team name is required")
		return
	}

	taskID, err := strconv.Atoi(parts[3])
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid task ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Action == "" {
		respondError(w, http.StatusBadRequest, "field 'action' is required")
		return
	}

	var task *agent.Task

	switch req.Action {
	case "cancel":
		task, err = agent.CancelTask(teamName, taskID)
	case "assign":
		if req.Owner == "" {
			respondError(w, http.StatusBadRequest, "field 'owner' is required for assign action")
			return
		}
		task, err = agent.AssignTask(teamName, taskID, req.Owner)
	case "redirect":
		if req.Instructions == "" {
			respondError(w, http.StatusBadRequest, "field 'instructions' is required for redirect action")
			return
		}
		task, err = agent.RedirectTask(teamName, taskID, req.Instructions, req.Subject)
	case "complete":
		task, err = agent.CompleteTask(teamName, taskID, req.Result)
	case "fail":
		task, err = agent.FailTask(teamName, taskID, req.Error)
	default:
		respondError(w, http.StatusBadRequest, fmt.Sprintf("unknown action: %s (valid: cancel, assign, redirect, complete, fail)", req.Action))
		return
	}

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("task not found: %v", err))
			return
		}
		respondError(w, http.StatusBadRequest, fmt.Sprintf("failed to %s task: %v", req.Action, err))
		return
	}

	respondJSON(w, http.StatusOK, taskToResponse(task))
}

// --- Message handlers ---

// handleListTeamMessages handles GET /teams/{name}/messages
func (s *HTTPServer) handleListTeamMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "messages" {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /teams/{name}/messages)")
		return
	}

	teamName := parts[1]
	if teamName == "" {
		respondError(w, http.StatusBadRequest, "team name is required")
		return
	}

	query := r.URL.Query()
	agentName := query.Get("agent")
	unreadOnly := query.Get("unread") == "true" || query.Get("unread") == "1"

	var messages []*agent.Message
	var err error

	if agentName != "" {
		messages, err = agent.GetMessages(teamName, agentName, unreadOnly)
	} else {
		limit := 50
		if limitStr := query.Get("limit"); limitStr != "" {
			if l, parseErr := strconv.Atoi(limitStr); parseErr == nil && l > 0 {
				limit = l
			}
		}
		messages, err = agent.GetAllTeamMessages(teamName, limit)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list messages: %v", err))
		return
	}

	resp := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		resp = append(resp, messageToResponse(m))
	}

	respondJSON(w, http.StatusOK, MessageListResponse{Messages: resp})
}

// handleSendTeamMessage handles POST /teams/{name}/messages
func (s *HTTPServer) handleSendTeamMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "messages" {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /teams/{name}/messages)")
		return
	}

	teamName := parts[1]
	if teamName == "" {
		respondError(w, http.StatusBadRequest, "team name is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.From == "" {
		respondError(w, http.StatusBadRequest, "field 'from' is required")
		return
	}
	if req.Content == "" {
		respondError(w, http.StatusBadRequest, "field 'content' is required")
		return
	}

	var msg *agent.Message
	var err error

	if req.To == "" {
		msg, err = agent.BroadcastMessage(teamName, req.From, req.Content)
	} else {
		msg, err = agent.SendMessage(teamName, req.From, req.To, req.Content)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to send message: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, messageToResponse(msg))
}

// --- Agent lifecycle handlers ---

// handleStartTeamAgents handles POST /teams/{name}/start
func (s *HTTPServer) handleStartTeamAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "start" {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /teams/{name}/start)")
		return
	}

	teamName := parts[1]
	if teamName == "" {
		respondError(w, http.StatusBadRequest, "team name is required")
		return
	}

	results, err := agent.StartAllAgents(teamName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("team not found: %v", err))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start agents: %v", err))
		return
	}

	resp := make([]AgentStartResponse, 0, len(results))
	for _, r := range results {
		resp = append(resp, AgentStartResponse{
			Name:    r.Name,
			Started: r.Started,
			PID:     r.PID,
			Error:   r.Error,
		})
	}

	respondJSON(w, http.StatusOK, StartTeamResponse{Results: resp})
}

// handleStopTeamAgents handles POST /teams/{name}/stop
func (s *HTTPServer) handleStopTeamAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "stop" {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /teams/{name}/stop)")
		return
	}

	teamName := parts[1]
	if teamName == "" {
		respondError(w, http.StatusBadRequest, "team name is required")
		return
	}

	team, err := agent.GetTeam(teamName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("team not found: %v", err))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get team: %v", err))
		return
	}

	resp := make([]AgentStopResponse, 0, len(team.Members))
	for _, m := range team.Members {
		result := AgentStopResponse{Name: m.Name}

		if !agent.IsAgentAlive(teamName, m.Name) {
			result.Stopped = true // already stopped
			resp = append(resp, result)
			continue
		}

		_, sendErr := agent.SendTypedMessage(teamName, agent.MsgSystem, "http-api", m.Name, "__stop__", 0)
		if sendErr != nil {
			result.Error = fmt.Sprintf("failed to send stop signal: %v", sendErr)
		} else {
			result.Stopped = true
		}

		resp = append(resp, result)
	}

	respondJSON(w, http.StatusOK, StopTeamResponse{Results: resp})
}

// --- Activity dashboard handler ---

// handleTeamActivity handles GET /teams/{name}/activity
func (s *HTTPServer) handleTeamActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "activity" {
		respondError(w, http.StatusBadRequest, "invalid path format (expected /teams/{name}/activity)")
		return
	}

	teamName := parts[1]
	if teamName == "" {
		respondError(w, http.StatusBadRequest, "team name is required")
		return
	}

	// 1. Get team config and member states
	team, err := agent.GetTeam(teamName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("team not found: %v", err))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get team: %v", err))
		return
	}

	members := make([]MemberActivity, 0, len(team.Members))
	for _, m := range team.Members {
		ma := MemberActivity{
			Name:   m.Name,
			Role:   m.Role,
			Model:  m.Model,
			Status: "stopped",
		}

		state, stateErr := agent.GetAgentState(teamName, m.Name)
		if stateErr == nil && state != nil {
			ma.Status = string(state.Status)
			ma.CurrentTask = state.CurrentTask
			ma.Activity = state.Activity
			ma.PID = state.PID
		}

		members = append(members, ma)
	}

	// 2. Get recent messages
	recentMsgs, err := agent.GetAllTeamMessages(teamName, 10)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get messages: %v", err))
		return
	}

	msgResp := make([]MessageResponse, 0, len(recentMsgs))
	for _, m := range recentMsgs {
		msgResp = append(msgResp, messageToResponse(m))
	}

	// 3. Compute task stats
	allTasks, err := agent.ListTasks(teamName, "", "")
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list tasks: %v", err))
		return
	}

	var stats TaskStats
	stats.Total = len(allTasks)
	for _, t := range allTasks {
		switch t.Status {
		case agent.TaskPending, agent.TaskAssigned:
			stats.Pending++
		case agent.TaskRunning:
			stats.Running++
		case agent.TaskCompleted:
			stats.Completed++
		case agent.TaskFailed:
			stats.Failed++
		}
	}

	respondJSON(w, http.StatusOK, TeamActivityResponse{
		Members:        members,
		RecentMessages: msgResp,
		TaskStats:      stats,
	})
}
