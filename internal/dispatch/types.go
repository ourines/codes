package dispatch

// DispatchOptions configures a dispatch request.
type DispatchOptions struct {
	UserInput   string // Raw user input text
	Channel     string // Source channel: "http", "telegram", "slack", etc.
	ChatID      string // Channel-specific chat/user ID
	Project     string // User-specified project (optional, dispatcher may override)
	Model       string // Anthropic model for intent analysis (default: haiku)
	CallbackURL string // URL to POST result when task completes (written at task creation time)
}

// IntentResponse is the structured output from Claude intent analysis.
type IntentResponse struct {
	Project string       `json:"project"`          // Matched project name
	Tasks   []TaskIntent `json:"tasks"`            // Parsed task list
	Clarify string       `json:"clarify,omitempty"` // Question to ask user if intent unclear
	Error   string       `json:"error,omitempty"`   // Error message if request is not actionable
}

// TaskIntent represents a single parsed task from the intent analysis.
type TaskIntent struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Priority    string `json:"priority,omitempty"`  // "high", "normal", "low"
	DependsOn   []int  `json:"dependsOn,omitempty"` // 1-based indices of tasks this depends on
}

// DispatchResult is returned after dispatch execution completes.
type DispatchResult struct {
	TeamName      string        `json:"teamName"`
	TasksCreated  int           `json:"tasksCreated"`
	AgentsStarted int           `json:"agentsStarted"`
	Intent        *IntentResponse `json:"intent,omitempty"`
	Clarify       string        `json:"clarify,omitempty"` // Forwarded from intent if unclear
	Error         string        `json:"error,omitempty"`
	DurationStr   string          `json:"duration"`
}
