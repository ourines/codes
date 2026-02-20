package httpserver

import "time"

// --- Session API Request Types ---

// CreateSessionRequest is the body for POST /sessions.
type CreateSessionRequest struct {
	ProjectName string `json:"project_name,omitempty"` // Registered project alias
	ProjectPath string `json:"project_path,omitempty"` // Explicit path (overrides project_name)
	Model       string `json:"model,omitempty"`        // Claude model (default: sonnet)
	Message     string `json:"message,omitempty"`      // First user message (optional)
}

// ResumeSessionRequest is the body for POST /sessions/{id}/resume.
type ResumeSessionRequest struct {
	ClaudeSessionID string `json:"claude_session_id"` // Claude session ID to resume
}

// SessionSendMessageRequest is the body for POST /sessions/{id}/message.
type SessionSendMessageRequest struct {
	Content string `json:"content"` // User message text
}

// --- Session API Response Types ---

// SessionResponse is the JSON shape for a single session.
type SessionResponse struct {
	ID              string  `json:"id"`
	ProjectName     string  `json:"project_name,omitempty"`
	ProjectPath     string  `json:"project_path"`
	Model           string  `json:"model,omitempty"`
	ClaudeSessionID string  `json:"claude_session_id,omitempty"`
	Status          string  `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	LastActiveAt    time.Time `json:"last_active_at"`
	CostUSD         float64 `json:"cost_usd"`
	TurnCount       int     `json:"turn_count"`
	ClientCount     int     `json:"client_count"`
}

// SessionListResponse wraps a list of sessions.
type SessionListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
}
