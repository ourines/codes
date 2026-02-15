package workflow

// Workflow defines a reusable sequence of Claude prompts.
type Workflow struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Steps       []Step `yaml:"steps" json:"steps"`
	BuiltIn     bool   `yaml:"-" json:"builtIn,omitempty"`
}

// Step is a single unit of work within a workflow.
type Step struct {
	Name            string `yaml:"name" json:"name"`
	Prompt          string `yaml:"prompt" json:"prompt"`
	WaitForApproval bool   `yaml:"wait_for_approval,omitempty" json:"waitForApproval,omitempty"`
}

// WorkflowRun tracks the execution state of a workflow.
type WorkflowRun struct {
	ID          string       `json:"id"`          // Unique run ID (timestamp-based)
	Workflow    *Workflow    `json:"workflow"`
	CurrentStep int          `json:"currentStep"`
	Status      string       `json:"status"` // "running", "paused", "completed", "failed", "aborted"
	Results     []StepResult `json:"results"`
	WorkDir     string       `json:"workDir,omitempty"`
	Model       string       `json:"model,omitempty"`
	StartedAt   string       `json:"startedAt,omitempty"`  // RFC3339 timestamp
	CompletedAt string       `json:"completedAt,omitempty"` // RFC3339 timestamp
}

// StepResult holds the output of a completed step.
type StepResult struct {
	StepName       string  `json:"stepName"`
	Result         string  `json:"result"`
	Cost           float64 `json:"cost,omitempty"`
	Error          string  `json:"error,omitempty"`
	ApprovalStatus string  `json:"approvalStatus,omitempty"` // "approved", "rejected", "skipped"
	Retries        int     `json:"retries,omitempty"`         // Number of retry attempts
}

// ApprovalDecision represents user's decision after reviewing a step result.
type ApprovalDecision string

const (
	ApprovalApprove ApprovalDecision = "approve" // Continue to next step
	ApprovalReject  ApprovalDecision = "reject"  // Retry current step
	ApprovalSkip    ApprovalDecision = "skip"    // Skip current step and continue
	ApprovalAbort   ApprovalDecision = "abort"   // Stop workflow execution
)
