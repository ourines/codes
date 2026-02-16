package agent

import "time"

// TaskStatus represents the state of a task.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskAssigned  TaskStatus = "assigned"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// AgentStatus represents the state of an agent daemon.
type AgentStatus string

const (
	AgentIdle     AgentStatus = "idle"
	AgentRunning  AgentStatus = "running"
	AgentStopping AgentStatus = "stopping"
	AgentStopped  AgentStatus = "stopped"
)

// TeamConfig holds the configuration for a team of agents.
type TeamConfig struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	WorkDir     string       `json:"workDir,omitempty"`
	Members     []TeamMember `json:"members"`
	CreatedAt   time.Time    `json:"createdAt"`
}

// TeamMember represents a registered agent in a team.
type TeamMember struct {
	Name  string `json:"name"`
	Role  string `json:"role,omitempty"`
	Model string `json:"model,omitempty"`
	Type  string `json:"type,omitempty"` // e.g. "worker", "leader"
}

// TaskPriority represents the urgency of a task.
type TaskPriority string

const (
	PriorityHigh   TaskPriority = "high"
	PriorityNormal TaskPriority = "normal"
	PriorityLow    TaskPriority = "low"
)

// Task represents a unit of work assigned to an agent.
type Task struct {
	ID          int          `json:"id"`
	Subject     string       `json:"subject"`
	Description string       `json:"description,omitempty"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority,omitempty"`
	Owner       string       `json:"owner,omitempty"`
	Project     string       `json:"project,omitempty"`  // registered project name for WorkDir resolution
	WorkDir     string       `json:"workDir,omitempty"`  // explicit working directory (overrides project)
	BlockedBy   []int        `json:"blockedBy,omitempty"`
	SessionID   string       `json:"sessionId,omitempty"`
	Adapter     string       `json:"adapter,omitempty"`   // CLI adapter to use (default: "claude")
	Result      string       `json:"result,omitempty"`
	Error       string       `json:"error,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
	CompletedAt *time.Time   `json:"completedAt,omitempty"`
}

// MessageType distinguishes different kinds of messages.
type MessageType string

const (
	MsgChat          MessageType = "chat"           // normal conversation
	MsgTaskCompleted MessageType = "task_completed"  // auto-report: task done
	MsgTaskFailed    MessageType = "task_failed"     // auto-report: task failed
	MsgSystem        MessageType = "system"          // system commands (__stop__, etc.)
)

// Message represents a message between agents.
type Message struct {
	ID        string      `json:"id"`
	Type      MessageType `json:"type"`
	From      string      `json:"from"`
	To        string      `json:"to"`                    // empty means broadcast
	Content   string      `json:"content"`
	TaskID    int         `json:"taskId,omitempty"`       // related task ID for reports
	Read      bool        `json:"read"`
	CreatedAt time.Time   `json:"createdAt"`
}

// AgentState represents the on-disk state of a running agent daemon.
type AgentState struct {
	Name         string      `json:"name"`
	Team         string      `json:"team"`
	PID          int         `json:"pid"`
	Status       AgentStatus `json:"status"`
	CurrentTask  int         `json:"currentTask,omitempty"`
	SessionID    string      `json:"sessionId,omitempty"` // persistent Claude session for message handling
	StartedAt    time.Time   `json:"startedAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
	RestartCount int         `json:"restartCount,omitempty"` // number of times daemon has been restarted
	LastCrash    *time.Time  `json:"lastCrash,omitempty"`    // timestamp of last crash/unexpected exit
	Supervised   bool        `json:"supervised,omitempty"`   // whether running under supervisor
}


// ClaudeResult holds the parsed output from a Claude CLI invocation.
type ClaudeResult struct {
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
	Duration  float64 `json:"duration_secs,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// RunOptions configures a Claude subprocess invocation.
type RunOptions struct {
	Prompt       string
	WorkDir      string
	SessionID    string
	Resume       bool
	Model        string
	SystemPrompt string
	AllowedTools []string
	MaxTurns     int
	PermMode     string // e.g. "dangerously-skip-permissions"
	Env          map[string]string
}
