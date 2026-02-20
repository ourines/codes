package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"codes/internal/chatsession"
	"codes/internal/config"
)

// handleCreateSession handles POST /sessions.
// Creates a new chat session and sends the first message.
func (s *HTTPServer) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	// Resolve project path.
	projectPath := req.ProjectPath
	projectName := req.ProjectName
	if projectPath == "" && projectName != "" {
		p, ok := config.GetProjectPath(projectName)
		if !ok {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("unknown project: %s", projectName))
			return
		}
		projectPath = p
	}
	if projectPath == "" {
		respondError(w, http.StatusBadRequest, "either 'project_path' or 'project_name' is required")
		return
	}

	session, err := chatsession.DefaultManager.Create(projectName, projectPath, req.Model)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create session: %v", err))
		return
	}

	if err := session.Start(req.Message); err != nil {
		chatsession.DefaultManager.Delete(session.ID)
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start session: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, sessionToResponse(session))
}

// handleListSessions handles GET /sessions.
func (s *HTTPServer) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	sessions := chatsession.DefaultManager.List()
	resp := SessionListResponse{
		Sessions: make([]SessionResponse, 0, len(sessions)),
	}
	for _, sess := range sessions {
		resp.Sessions = append(resp.Sessions, sessionToResponse(sess))
	}

	respondJSON(w, http.StatusOK, resp)
}

// handleGetSession handles GET /sessions/{id}.
func (s *HTTPServer) handleGetSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := extractSessionID(r.URL.Path)
	if id == "" {
		respondError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	session, ok := chatsession.DefaultManager.Get(id)
	if !ok {
		respondError(w, http.StatusNotFound, fmt.Sprintf("session %s not found", id))
		return
	}

	respondJSON(w, http.StatusOK, sessionToResponse(session))
}

// handleDeleteSession handles DELETE /sessions/{id}.
func (s *HTTPServer) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := extractSessionID(r.URL.Path)
	if id == "" {
		respondError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	if err := chatsession.DefaultManager.Delete(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete session: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleInterruptSession handles POST /sessions/{id}/interrupt.
func (s *HTTPServer) handleInterruptSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := extractSessionIDFromAction(r.URL.Path, "interrupt")
	if id == "" {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	session, ok := chatsession.DefaultManager.Get(id)
	if !ok {
		respondError(w, http.StatusNotFound, fmt.Sprintf("session %s not found", id))
		return
	}

	if err := session.Interrupt(); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("interrupt failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "interrupted"})
}

// handleResumeSession handles POST /sessions/{id}/resume.
func (s *HTTPServer) handleResumeSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req ResumeSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.ClaudeSessionID == "" {
		respondError(w, http.StatusBadRequest, "field 'claude_session_id' is required")
		return
	}

	// Extract optional project info from path or use defaults.
	id := extractSessionIDFromAction(r.URL.Path, "resume")
	if id == "" {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Look up existing session to get project info.
	existing, ok := chatsession.DefaultManager.Get(id)
	if !ok {
		respondError(w, http.StatusNotFound, fmt.Sprintf("session %s not found", id))
		return
	}

	info := existing.Snapshot()

	// Close the old session and create a resumed one.
	chatsession.DefaultManager.Delete(id)

	resumed, err := chatsession.DefaultManager.Resume(
		req.ClaudeSessionID, info.ProjectName, info.ProjectPath, info.Model,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("resume failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, sessionToResponse(resumed))
}

// handleSessionWebSocket handles WS /sessions/{id}/ws.
func (s *HTTPServer) handleSessionWebSocket(w http.ResponseWriter, r *http.Request) {
	id := extractSessionIDFromAction(r.URL.Path, "ws")
	if id == "" {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	session, ok := chatsession.DefaultManager.Get(id)
	if !ok {
		respondError(w, http.StatusNotFound, fmt.Sprintf("session %s not found", id))
		return
	}

	chatsession.HandleWebSocket(session, w, r)
}

// handleSessionMessage handles POST /sessions/{id}/message.
func (s *HTTPServer) handleSessionMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := extractSessionIDFromAction(r.URL.Path, "message")
	if id == "" {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	session, ok := chatsession.DefaultManager.Get(id)
	if !ok {
		respondError(w, http.StatusNotFound, fmt.Sprintf("session %s not found", id))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req SessionSendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Content == "" {
		respondError(w, http.StatusBadRequest, "field 'content' is required")
		return
	}

	if err := session.SendMessage(req.Content); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("send message failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, sessionToResponse(session))
}

// --- helpers ---

// extractSessionID extracts the session ID from "/sessions/{id}".
func extractSessionID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 2 && parts[0] == "sessions" {
		return parts[1]
	}
	return ""
}

// extractSessionIDFromAction extracts the session ID from "/sessions/{id}/{action}".
func extractSessionIDFromAction(path, action string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 3 && parts[0] == "sessions" && parts[2] == action {
		return parts[1]
	}
	return ""
}

// sessionToResponse converts a ChatSession to the API response type.
func sessionToResponse(s *chatsession.ChatSession) SessionResponse {
	info := s.Snapshot()
	return SessionResponse{
		ID:              info.ID,
		ProjectName:     info.ProjectName,
		ProjectPath:     info.ProjectPath,
		Model:           info.Model,
		ClaudeSessionID: info.ClaudeSessionID,
		Status:          string(info.Status),
		CreatedAt:       info.CreatedAt,
		LastActiveAt:    info.LastActiveAt,
		CostUSD:         info.CostUSD,
		TurnCount:       info.TurnCount,
		ClientCount:     info.ClientCount,
	}
}
