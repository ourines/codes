package workflow

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ApprovalPrompt prompts the user for approval of a workflow step.
type ApprovalPrompt interface {
	// PromptApproval asks the user to approve, reject, skip, or abort.
	// Returns the decision and any error from I/O.
	PromptApproval(stepName string, result *StepResult) (ApprovalDecision, error)
}

// CLIApprovalPrompt implements interactive CLI-based approval.
type CLIApprovalPrompt struct {
	reader *bufio.Reader
}

// NewCLIApprovalPrompt creates a new CLI approval prompter.
func NewCLIApprovalPrompt() *CLIApprovalPrompt {
	return &CLIApprovalPrompt{
		reader: bufio.NewReader(os.Stdin),
	}
}

// PromptApproval prompts the user interactively for their decision.
func (p *CLIApprovalPrompt) PromptApproval(stepName string, result *StepResult) (ApprovalDecision, error) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("📋 Step: %s\n", stepName)
	fmt.Println(strings.Repeat("=", 70))

	if result.Error != "" {
		fmt.Printf("❌ Error: %s\n\n", result.Error)
	} else {
		fmt.Println("✅ Step completed successfully")
	}

	if result.Result != "" {
		fmt.Println("\nResult:")
		fmt.Println(strings.Repeat("-", 70))
		fmt.Println(result.Result)
		fmt.Println(strings.Repeat("-", 70))
	}

	if result.Cost > 0 {
		fmt.Printf("\n💰 Cost: $%.4f\n", result.Cost)
	}

	fmt.Println("\nOptions:")
	if result.Error != "" {
		fmt.Println("  [a] approve  - Accept result and continue")
		fmt.Println("  [r] reject   - Retry this step")
		fmt.Println("  [s] skip     - Skip this step and continue")
		fmt.Println("  [x] abort    - Stop workflow execution")
	} else {
		fmt.Println("  [a] approve  - Continue to next step")
		fmt.Println("  [r] reject   - Retry this step")
		fmt.Println("  [s] skip     - Skip to next step")
		fmt.Println("  [x] abort    - Stop workflow")
	}

	for {
		fmt.Print("\nYour decision [a/r/s/x]: ")
		input, err := p.reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read input: %w", err)
		}

		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "a", "approve":
			return ApprovalApprove, nil
		case "r", "reject":
			return ApprovalReject, nil
		case "s", "skip":
			return ApprovalSkip, nil
		case "x", "abort":
			return ApprovalAbort, nil
		default:
			fmt.Printf("Invalid input: '%s'. Please enter a, r, s, or x.\n", input)
		}
	}
}

// AutoApprovalPrompt automatically approves all steps (for non-interactive use).
type AutoApprovalPrompt struct{}

// NewAutoApprovalPrompt creates an auto-approval prompter.
func NewAutoApprovalPrompt() *AutoApprovalPrompt {
	return &AutoApprovalPrompt{}
}

// PromptApproval automatically approves unless there's an error.
func (p *AutoApprovalPrompt) PromptApproval(stepName string, result *StepResult) (ApprovalDecision, error) {
	if result.Error != "" {
		return ApprovalAbort, nil // Fail fast on errors
	}
	return ApprovalApprove, nil
}
