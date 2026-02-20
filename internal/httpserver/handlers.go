package httpserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"codes/internal/agent"
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

	task, err := agent.GetTask(teamName, taskID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("task not found: %v", err))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get task: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, TaskResponse{
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
	})
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

	summaries := make([]TeamSummary, 0, len(teamNames))
	for _, name := range teamNames {
		team, err := agent.GetTeam(name)
		if err != nil {
			continue
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

	team, err := agent.GetTeam(teamName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("team not found: %v", err))
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get team: %v", err))
		return
	}

	members := make([]TeamMember, 0, len(team.Members))
	for _, m := range team.Members {
		member := TeamMember{
			Name:  m.Name,
			Role:  m.Role,
			Model: m.Model,
			Type:  m.Type,
		}
		state, err := agent.GetAgentState(teamName, m.Name)
		if err == nil && state != nil {
			member.Status = string(state.Status)
			member.PID = state.PID
		} else {
			member.Status = "stopped"
		}
		members = append(members, member)
	}

	respondJSON(w, http.StatusOK, TeamDetailResponse{
		Name:        team.Name,
		Description: team.Description,
		WorkDir:     team.WorkDir,
		Members:     members,
		CreatedAt:   team.CreatedAt,
	})
}
