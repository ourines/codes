package commands

import (
	"fmt"
	"os"

	"codes/internal/ui"
	"codes/internal/workflow"
)

// RunWorkflowList displays all available workflows.
func RunWorkflowList() {
	workflows, err := workflow.ListWorkflows()
	if err != nil {
		ui.ShowError("Failed to list workflows", err)
		return
	}

	if len(workflows) == 0 {
		fmt.Println("No workflows found.")
		return
	}

	fmt.Println()
	for _, wf := range workflows {
		tag := ""
		if wf.BuiltIn {
			tag = " (built-in)"
		}
		fmt.Printf("  %s%s\n", wf.Name, tag)
		if wf.Description != "" {
			fmt.Printf("    %s\n", wf.Description)
		}
		fmt.Printf("    Agents: %d  Tasks: %d\n", len(wf.Agents), len(wf.Tasks))
		fmt.Println()
	}
}

// RunWorkflowRun launches a workflow as an agent team (non-blocking).
func RunWorkflowRun(name, dir, model, project string) {
	wf, err := workflow.GetWorkflow(name)
	if err != nil {
		ui.ShowError("Workflow not found", err)
		return
	}

	if dir == "" {
		dir, _ = os.Getwd()
	}

	fmt.Printf("Launching workflow: %s (%d agents, %d tasks)\n", wf.Name, len(wf.Agents), len(wf.Tasks))
	if wf.Description != "" {
		fmt.Printf("  %s\n", wf.Description)
	}
	fmt.Println()

	result, err := workflow.RunWorkflow(wf, workflow.RunWorkflowOptions{
		WorkDir: dir,
		Model:   model,
		Project: project,
	})
	if err != nil {
		ui.ShowError("Workflow launch failed", err)
		return
	}

	ui.ShowSuccess("Workflow launched as team: %s", result.TeamName)
	fmt.Printf("  Agents started: %d\n", result.Agents)
	fmt.Printf("  Tasks created:  %d\n", result.Tasks)
	fmt.Println()
	fmt.Printf("Monitor progress: codes agent status %s\n", result.TeamName)
}

// RunWorkflowCreate creates a new workflow template file.
func RunWorkflowCreate(name string) {
	// Check if already exists
	if _, err := workflow.GetWorkflow(name); err == nil {
		ui.ShowError(fmt.Sprintf("Workflow %q already exists", name), nil)
		return
	}

	wf := &workflow.Workflow{
		Name:        name,
		Description: "Custom workflow",
		Agents: []workflow.WorkflowAgent{
			{Name: "worker", Role: "Execute workflow tasks"},
		},
		Tasks: []workflow.WorkflowTask{
			{
				Subject: "Task 1",
				Assign:  "worker",
				Prompt:  "Describe what this task should do",
			},
		},
	}

	if err := workflow.SaveWorkflow(wf); err != nil {
		ui.ShowError("Failed to create workflow", err)
		return
	}

	fmt.Printf("Created workflow template: %s\n", workflow.WorkflowDir()+"/"+name+".yml")
	fmt.Println("Edit the YAML file to customize agents and tasks.")
}

// RunWorkflowDelete removes a workflow.
func RunWorkflowDelete(name string) {
	if err := workflow.DeleteWorkflow(name); err != nil {
		ui.ShowError("Failed to delete workflow", err)
		return
	}
	ui.ShowSuccess("Deleted workflow: %s", name)
}
