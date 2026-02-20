package httpserver

import "time"

// --- Request types for Block D endpoints ---

// CreateTeamRequest is the request body for POST /teams.
type CreateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	WorkDir     string `json:"work_dir,omitempty"`
}

// CreateTaskRequest is the request body for POST /teams/{name}/tasks.
type CreateTaskRequest struct {
	Subject     string `json:"subject"`
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner,omitempty"`
	Priority    string `json:"priority,omitempty"`
	BlockedBy   []int  `json:"blocked_by,omitempty"`
	Project     string `json:"project,omitempty"`
	WorkDir     string `json:"work_dir,omitempty"`
}

// UpdateTaskRequest is the request body for PATCH /teams/{name}/tasks/{id}.
type UpdateTaskRequest struct {
	Action       string `json:"action"` // "cancel", "assign", "redirect", "complete", "fail"
	Owner        string `json:"owner,omitempty"`
	Subject      string `json:"subject,omitempty"`
	Instructions string `json:"instructions,omitempty"`
	Result       string `json:"result,omitempty"`
	Error        string `json:"error,omitempty"`
}

// SendMessageRequest is the request body for POST /teams/{name}/messages.
type SendMessageRequest struct {
	From    string `json:"from"`
	To      string `json:"to,omitempty"` // empty = broadcast
	Content string `json:"content"`
}

// --- Response types for Block D endpoints ---

// TaskListResponse wraps a list of tasks.
type TaskListResponse struct {
	Tasks []TaskResponse `json:"tasks"`
}

// MessageListResponse wraps a list of messages.
type MessageListResponse struct {
	Messages []MessageResponse `json:"messages"`
}

// MessageResponse represents a message in the HTTP response.
type MessageResponse struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	From      string    `json:"from"`
	To        string    `json:"to,omitempty"`
	Content   string    `json:"content"`
	TaskID    int       `json:"task_id,omitempty"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// TeamActivityResponse represents the team activity dashboard.
type TeamActivityResponse struct {
	Members        []MemberActivity  `json:"members"`
	RecentMessages []MessageResponse `json:"recent_messages"`
	TaskStats      TaskStats         `json:"task_stats"`
}

// MemberActivity represents an agent's current activity in the team dashboard.
type MemberActivity struct {
	Name        string `json:"name"`
	Role        string `json:"role,omitempty"`
	Model       string `json:"model,omitempty"`
	Status      string `json:"status"`
	CurrentTask int    `json:"current_task,omitempty"`
	Activity    string `json:"activity,omitempty"`
	PID         int    `json:"pid,omitempty"`
}

// TaskStats summarizes task counts by status.
type TaskStats struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// StartTeamResponse is returned by POST /teams/{name}/start.
type StartTeamResponse struct {
	Results []AgentStartResponse `json:"results"`
}

// AgentStartResponse represents the result of starting a single agent.
type AgentStartResponse struct {
	Name    string `json:"name"`
	Started bool   `json:"started"`
	PID     int    `json:"pid,omitempty"`
	Error   string `json:"error,omitempty"`
}

// StopTeamResponse is returned by POST /teams/{name}/stop.
type StopTeamResponse struct {
	Results []AgentStopResponse `json:"results"`
}

// AgentStopResponse represents the result of stopping a single agent.
type AgentStopResponse struct {
	Name    string `json:"name"`
	Stopped bool   `json:"stopped"`
	Error   string `json:"error,omitempty"`
}
