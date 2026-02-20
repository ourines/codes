package httpserver

// WorkflowListResponse represents the list of workflows.
type WorkflowListResponse struct {
	Workflows []WorkflowSummary `json:"workflows"`
}

// WorkflowSummary represents a workflow in list responses.
type WorkflowSummary struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	AgentCount  int    `json:"agent_count"`
	TaskCount   int    `json:"task_count"`
	BuiltIn     bool   `json:"built_in"`
}

// RunWorkflowRequest represents a request to run a workflow.
type RunWorkflowRequest struct {
	WorkDir string `json:"work_dir,omitempty"`
	Model   string `json:"model,omitempty"`
	Project string `json:"project,omitempty"`
}

// RunWorkflowResponse represents the result of running a workflow.
type RunWorkflowResponse struct {
	TeamName      string `json:"team_name"`
	AgentsStarted int    `json:"agents_started"`
	TasksCreated  int    `json:"tasks_created"`
}
