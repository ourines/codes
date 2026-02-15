package commands

import (
	"context"
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
		fmt.Printf("    Steps: %d\n", len(wf.Steps))
		fmt.Println()
	}
}

// RunWorkflowRun executes a workflow by name.
func RunWorkflowRun(name, dir, model string) {
	wf, err := workflow.GetWorkflow(name)
	if err != nil {
		ui.ShowError("Workflow not found", err)
		return
	}

	if dir == "" {
		dir, _ = os.Getwd()
	}

	fmt.Printf("Running workflow: %s (%d steps)\n", wf.Name, len(wf.Steps))
	if wf.Description != "" {
		fmt.Printf("  %s\n", wf.Description)
	}
	fmt.Println()

	ctx := context.Background()
	run, err := workflow.RunWorkflow(ctx, wf, dir, model)
	if err != nil {
		ui.ShowError("Workflow execution failed", err)
		return
	}

	// Print results
	for i, result := range run.Results {
		status := "✓"
		if result.Error != "" {
			status = "✗"
		}
		fmt.Printf("  %s Step %d: %s\n", status, i+1, result.StepName)
		if result.Error != "" {
			fmt.Printf("    Error: %s\n", result.Error)
		}
		if result.Cost > 0 {
			fmt.Printf("    Cost: $%.4f\n", result.Cost)
		}
		fmt.Println()
	}

	if run.Status == "completed" {
		ui.ShowSuccess("Workflow completed successfully.")
	} else {
		ui.ShowError(fmt.Sprintf("Workflow %s", run.Status), nil)
	}
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
		Steps: []workflow.Step{
			{
				Name:   "Step 1",
				Prompt: "Describe what this step should do",
			},
		},
	}

	if err := workflow.SaveWorkflow(wf); err != nil {
		ui.ShowError("Failed to create workflow", err)
		return
	}

	fmt.Printf("Created workflow template: %s\n", workflow.WorkflowDir()+"/"+name+".yml")
	fmt.Println("Edit the YAML file to customize steps.")
}

// RunWorkflowDelete removes a workflow.
func RunWorkflowDelete(name string) {
	if err := workflow.DeleteWorkflow(name); err != nil {
		ui.ShowError("Failed to delete workflow", err)
		return
	}
	ui.ShowSuccess("Deleted workflow: %s", name)
}
