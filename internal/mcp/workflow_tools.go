package mcpserver

import (
	"context"
	"fmt"
	"os"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"codes/internal/workflow"
)

// -- workflow_list --

type workflowListInput struct{}

type workflowListOutput struct {
	Workflows []workflow.Workflow `json:"workflows"`
}

func workflowListHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input workflowListInput) (*mcpsdk.CallToolResult, workflowListOutput, error) {
	workflows, err := workflow.ListWorkflows()
	if err != nil {
		return nil, workflowListOutput{}, err
	}
	if workflows == nil {
		workflows = []workflow.Workflow{}
	}
	return nil, workflowListOutput{Workflows: workflows}, nil
}

// -- workflow_get --

type workflowGetInput struct {
	Name string `json:"name" jsonschema:"Workflow name"`
}

type workflowGetOutput struct {
	Workflow *workflow.Workflow `json:"workflow"`
}

func workflowGetHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input workflowGetInput) (*mcpsdk.CallToolResult, workflowGetOutput, error) {
	if input.Name == "" {
		return nil, workflowGetOutput{}, fmt.Errorf("name is required")
	}
	wf, err := workflow.GetWorkflow(input.Name)
	if err != nil {
		return nil, workflowGetOutput{}, err
	}
	return nil, workflowGetOutput{Workflow: wf}, nil
}

// -- workflow_run --

type workflowRunInput struct {
	Name    string `json:"name" jsonschema:"Workflow name to run"`
	WorkDir string `json:"workDir,omitempty" jsonschema:"Working directory (default: current directory)"`
	Model   string `json:"model,omitempty" jsonschema:"Claude model to use"`
	Project string `json:"project,omitempty" jsonschema:"Project name to execute in (registered via add_project)"`
}

type workflowRunOutput struct {
	TeamName     string `json:"teamName"`
	AgentsStarted int   `json:"agentsStarted"`
	TasksCreated  int   `json:"tasksCreated"`
	MonitorHint  string `json:"monitorHint"`
}

func workflowRunHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input workflowRunInput) (*mcpsdk.CallToolResult, workflowRunOutput, error) {
	if input.Name == "" {
		return nil, workflowRunOutput{}, fmt.Errorf("name is required")
	}
	wf, err := workflow.GetWorkflow(input.Name)
	if err != nil {
		return nil, workflowRunOutput{}, err
	}
	workDir := input.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	result, err := workflow.RunWorkflow(wf, workflow.RunWorkflowOptions{
		WorkDir: workDir,
		Model:   input.Model,
		Project: input.Project,
	})
	if err != nil {
		return nil, workflowRunOutput{}, err
	}
	return nil, workflowRunOutput{
		TeamName:      result.TeamName,
		AgentsStarted: result.Agents,
		TasksCreated:  result.Tasks,
		MonitorHint:   fmt.Sprintf("Use team_status with name=%q to monitor progress", result.TeamName),
	}, nil
}

// -- workflow_create --

type workflowCreateInput struct {
	Name        string                  `json:"name" jsonschema:"Workflow name (used as filename)"`
	Description string                  `json:"description,omitempty" jsonschema:"Workflow description"`
	Agents      []workflow.WorkflowAgent `json:"agents" jsonschema:"Agent definitions"`
	Tasks       []workflow.WorkflowTask  `json:"tasks" jsonschema:"Task definitions"`
}

type workflowCreateOutput struct {
	Created  bool   `json:"created"`
	FilePath string `json:"filePath"`
}

func workflowCreateHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input workflowCreateInput) (*mcpsdk.CallToolResult, workflowCreateOutput, error) {
	if input.Name == "" {
		return nil, workflowCreateOutput{}, fmt.Errorf("name is required")
	}
	if len(input.Agents) == 0 {
		return nil, workflowCreateOutput{}, fmt.Errorf("at least one agent is required")
	}
	if len(input.Tasks) == 0 {
		return nil, workflowCreateOutput{}, fmt.Errorf("at least one task is required")
	}

	// Check if already exists
	if existing, _ := workflow.GetWorkflow(input.Name); existing != nil {
		return nil, workflowCreateOutput{}, fmt.Errorf("workflow %q already exists", input.Name)
	}

	// Validate task.Assign references existing agents
	agentNames := make(map[string]bool)
	for _, a := range input.Agents {
		if a.Name == "" {
			return nil, workflowCreateOutput{}, fmt.Errorf("agent name cannot be empty")
		}
		agentNames[a.Name] = true
	}
	for i, t := range input.Tasks {
		if t.Subject == "" {
			return nil, workflowCreateOutput{}, fmt.Errorf("task %d subject cannot be empty", i+1)
		}
		if t.Assign != "" && !agentNames[t.Assign] {
			return nil, workflowCreateOutput{}, fmt.Errorf("task %d (%q) assigns to unknown agent %q", i+1, t.Subject, t.Assign)
		}
		for _, dep := range t.BlockedBy {
			if dep < 1 || dep > len(input.Tasks) {
				return nil, workflowCreateOutput{}, fmt.Errorf("task %d (%q) has invalid blockedBy index %d (must be 1-%d)", i+1, t.Subject, dep, len(input.Tasks))
			}
		}
	}

	wf := &workflow.Workflow{
		Name:        input.Name,
		Description: input.Description,
		Agents:      input.Agents,
		Tasks:       input.Tasks,
	}

	if err := workflow.SaveWorkflow(wf); err != nil {
		return nil, workflowCreateOutput{}, fmt.Errorf("save workflow: %w", err)
	}

	return nil, workflowCreateOutput{
		Created:  true,
		FilePath: workflow.WorkflowDir() + "/" + input.Name + ".yml",
	}, nil
}

// registerWorkflowTools registers workflow-related MCP tools.
func registerWorkflowTools(server *mcpsdk.Server) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "workflow_list",
		Description: "List all available workflow templates (built-in and custom)",
	}, workflowListHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "workflow_get",
		Description: "Get details of a specific workflow including all steps",
	}, workflowGetHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "workflow_run",
		Description: "Execute a workflow by name, running all steps sequentially",
	}, workflowRunHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "workflow_create",
		Description: "Create a new workflow template with agents and tasks. Validates that task assignments reference defined agents and blockedBy indices are valid.",
	}, workflowCreateHandler)
}
