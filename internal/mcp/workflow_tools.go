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
}

type workflowRunOutput struct {
	Run *workflow.WorkflowRun `json:"run"`
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
	run, err := workflow.RunWorkflow(ctx, wf, workDir, input.Model)
	if err != nil {
		return nil, workflowRunOutput{}, err
	}
	return nil, workflowRunOutput{Run: run}, nil
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
}
