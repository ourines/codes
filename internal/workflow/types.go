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
	Workflow    *Workflow    `json:"workflow"`
	CurrentStep int          `json:"currentStep"`
	Status      string       `json:"status"` // "running", "paused", "completed", "failed"
	Results     []StepResult `json:"results"`
}

// StepResult holds the output of a completed step.
type StepResult struct {
	StepName string  `json:"stepName"`
	Result   string  `json:"result"`
	Cost     float64 `json:"cost,omitempty"`
	Error    string  `json:"error,omitempty"`
}
