package workflow

import (
	"context"
	"fmt"
	"time"

	"codes/internal/agent"
)

// RunWorkflowOptions configures workflow execution.
type RunWorkflowOptions struct {
	WorkDir        string
	Model          string
	ApprovalPrompt ApprovalPrompt // If nil, uses auto-approval
	MaxRetries     int            // Max retries per step (default: 3)
}

// RunWorkflow executes a workflow sequentially, step by step.
func RunWorkflow(ctx context.Context, wf *Workflow, opts RunWorkflowOptions) (*WorkflowRun, error) {
	// Set defaults
	if opts.ApprovalPrompt == nil {
		opts.ApprovalPrompt = NewAutoApprovalPrompt()
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}

	run := &WorkflowRun{
		Workflow:    wf,
		CurrentStep: 0,
		Status:      "running",
		WorkDir:     opts.WorkDir,
		Model:       opts.Model,
		StartedAt:   time.Now().Format(time.RFC3339),
	}

	// Save initial state
	if err := SaveRun(run); err != nil {
		// Non-fatal: log but continue
		fmt.Printf("Warning: failed to save workflow run: %v\n", err)
	}

	for i := 0; i < len(wf.Steps); {
		step := wf.Steps[i]
		run.CurrentStep = i

		stepResult, shouldContinue := executeStepWithRetry(ctx, run, step, i, opts)

		run.Results = append(run.Results, stepResult)

		// Save progress after each step
		if err := SaveRun(run); err != nil {
			fmt.Printf("Warning: failed to save workflow run: %v\n", err)
		}

		if !shouldContinue {
			run.CompletedAt = time.Now().Format(time.RFC3339)
			if err := SaveRun(run); err != nil {
				fmt.Printf("Warning: failed to save final workflow run: %v\n", err)
			}
			return run, nil
		}

		i++ // Move to next step
	}

	run.Status = "completed"
	run.CompletedAt = time.Now().Format(time.RFC3339)
	if err := SaveRun(run); err != nil {
		fmt.Printf("Warning: failed to save final workflow run: %v\n", err)
	}
	return run, nil
}

// executeStepWithRetry executes a single step with retry logic and approval.
// Returns (stepResult, shouldContinue).
func executeStepWithRetry(
	ctx context.Context,
	run *WorkflowRun,
	step Step,
	stepIndex int,
	opts RunWorkflowOptions,
) (StepResult, bool) {
	stepResult := StepResult{
		StepName: step.Name,
	}

	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		// Build prompt with context from previous steps
		prompt := step.Prompt
		if stepIndex > 0 && len(run.Results) > 0 {
			prev := run.Results[len(run.Results)-1]
			prompt = fmt.Sprintf("Previous step (%s) result:\n%s\n\nNow: %s",
				prev.StepName, prev.Result, step.Prompt)
		}

		result, err := agent.RunClaude(ctx, agent.RunOptions{
			Prompt:   prompt,
			WorkDir:  opts.WorkDir,
			Model:    opts.Model,
			PermMode: "dangerously-skip-permissions",
		})

		// Handle execution error
		if err != nil {
			stepResult.Error = err.Error()
			stepResult.Retries = attempt
		} else if result.IsError {
			stepResult.Error = result.Error
			stepResult.Result = result.Result
			stepResult.Retries = attempt
		} else {
			// Success
			stepResult.Result = result.Result
			stepResult.Cost = result.CostUSD
			stepResult.Retries = attempt
		}

		// If no approval needed, return immediately
		if !step.WaitForApproval {
			if stepResult.Error != "" {
				run.Status = "failed"
				return stepResult, false // Stop on error without approval
			}
			stepResult.ApprovalStatus = "auto-approved"
			return stepResult, true
		}

		// Prompt for approval
		decision, err := opts.ApprovalPrompt.PromptApproval(step.Name, &stepResult)
		if err != nil {
			// I/O error during approval prompt
			stepResult.Error = fmt.Sprintf("approval prompt error: %v", err)
			run.Status = "failed"
			return stepResult, false
		}

		switch decision {
		case ApprovalApprove:
			stepResult.ApprovalStatus = "approved"
			return stepResult, true

		case ApprovalSkip:
			stepResult.ApprovalStatus = "skipped"
			stepResult.Result = "(skipped by user)"
			return stepResult, true

		case ApprovalAbort:
			stepResult.ApprovalStatus = "aborted"
			run.Status = "aborted"
			return stepResult, false

		case ApprovalReject:
			stepResult.ApprovalStatus = "rejected"
			if attempt < opts.MaxRetries {
				fmt.Printf("\n🔄 Retrying step (attempt %d/%d)...\n\n", attempt+2, opts.MaxRetries+1)
				continue // Retry
			} else {
				fmt.Printf("\n❌ Max retries (%d) reached. Marking step as failed.\n", opts.MaxRetries)
				run.Status = "failed"
				return stepResult, false
			}
		}
	}

	// Should not reach here
	run.Status = "failed"
	return stepResult, false
}
