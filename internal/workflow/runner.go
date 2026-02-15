package workflow

import (
	"context"
	"fmt"

	"codes/internal/agent"
)

// RunWorkflow executes a workflow sequentially, step by step.
func RunWorkflow(ctx context.Context, wf *Workflow, workDir, model string) (*WorkflowRun, error) {
	run := &WorkflowRun{
		Workflow:    wf,
		CurrentStep: 0,
		Status:      "running",
	}

	for i, step := range wf.Steps {
		run.CurrentStep = i

		// Build prompt with context from previous steps
		prompt := step.Prompt
		if i > 0 && len(run.Results) > 0 {
			prev := run.Results[len(run.Results)-1]
			prompt = fmt.Sprintf("Previous step (%s) result:\n%s\n\nNow: %s",
				prev.StepName, prev.Result, step.Prompt)
		}

		result, err := agent.RunClaude(ctx, agent.RunOptions{
			Prompt:   prompt,
			WorkDir:  workDir,
			Model:    model,
			PermMode: "dangerously-skip-permissions",
		})

		stepResult := StepResult{
			StepName: step.Name,
		}

		if err != nil {
			stepResult.Error = err.Error()
			run.Results = append(run.Results, stepResult)
			run.Status = "failed"
			return run, nil
		}

		if result.IsError {
			stepResult.Error = result.Error
			stepResult.Result = result.Result
			run.Results = append(run.Results, stepResult)
			run.Status = "failed"
			return run, nil
		}

		stepResult.Result = result.Result
		stepResult.Cost = result.CostUSD
		run.Results = append(run.Results, stepResult)
	}

	run.Status = "completed"
	return run, nil
}
