package workflow

// Workflow defines a reusable agent team template.
type Workflow struct {
	Name        string          `yaml:"name" json:"name"`
	Description string          `yaml:"description,omitempty" json:"description,omitempty"`
	Agents      []WorkflowAgent `yaml:"agents" json:"agents"`
	Tasks       []WorkflowTask  `yaml:"tasks" json:"tasks"`
	BuiltIn     bool            `yaml:"-" json:"builtIn,omitempty"`
}

// WorkflowAgent defines an agent within a workflow.
type WorkflowAgent struct {
	Name  string `yaml:"name" json:"name"`
	Role  string `yaml:"role,omitempty" json:"role,omitempty"`
	Model string `yaml:"model,omitempty" json:"model,omitempty"`
}

// WorkflowTask defines a task to be created when the workflow runs.
type WorkflowTask struct {
	Subject   string `yaml:"subject" json:"subject"`
	Assign    string `yaml:"assign,omitempty" json:"assign,omitempty"`
	Prompt    string `yaml:"prompt" json:"prompt"`
	Priority  string `yaml:"priority,omitempty" json:"priority,omitempty"`
	BlockedBy []int  `yaml:"blocked_by,omitempty" json:"blockedBy,omitempty"` // 1-based index into Tasks
}

// WorkflowRunResult holds the result of launching a workflow as an agent team.
type WorkflowRunResult struct {
	TeamName string `json:"teamName"`
	Agents   int    `json:"agentsStarted"`
	Tasks    int    `json:"tasksCreated"`
}
