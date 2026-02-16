package httpserver

import "time"

// DispatchRequest represents the incoming dispatch API request
type DispatchRequest struct {
	Text     string `json:"text"`               // User's request text
	Channel  string `json:"channel,omitempty"`  // Source channel (e.g., "telegram", "slack")
	ChatID   string `json:"chat_id,omitempty"`  // Channel-specific chat/user ID
	Project  string `json:"project,omitempty"`  // Target project name
	Priority string `json:"priority,omitempty"` // Task priority: "high", "normal", "low"
}

// DispatchResponse represents the dispatch API response
type DispatchResponse struct {
	TaskID int    `json:"task_id"` // Created task ID
	Team   string `json:"team"`    // Created team name
	Status string `json:"status"`  // Task status (typically "pending")
}

// TaskResponse represents the task status response
type TaskResponse struct {
	ID          int       `json:"id"`
	Subject     string    `json:"subject"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	Project     string    `json:"project,omitempty"`
	WorkDir     string    `json:"work_dir,omitempty"`
	Result      string    `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TeamListResponse represents the teams list response
type TeamListResponse struct {
	Teams []TeamSummary `json:"teams"`
}

// TeamSummary represents a summary of a team
type TeamSummary struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// TeamDetailResponse represents detailed team information
type TeamDetailResponse struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	WorkDir     string        `json:"work_dir,omitempty"`
	Members     []TeamMember  `json:"members"`
	CreatedAt   time.Time     `json:"created_at"`
}

// TeamMember represents a team member with status
type TeamMember struct {
	Name   string `json:"name"`
	Role   string `json:"role,omitempty"`
	Model  string `json:"model,omitempty"`
	Type   string `json:"type,omitempty"`
	Status string `json:"status,omitempty"` // Agent status: "idle", "running", "stopped"
	PID    int    `json:"pid,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}
