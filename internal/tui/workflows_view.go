package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codes/internal/workflow"
)

// workflowsLoadedMsg is sent after loading workflows.
type workflowsLoadedMsg struct {
	workflows []workflow.Workflow
	err       error
}

// workflowRunMsg is sent after a workflow run completes.
type workflowRunMsg struct {
	run *workflow.WorkflowRun
	err error
}

// loadWorkflowsCmd loads all workflows asynchronously.
func loadWorkflowsCmd() tea.Cmd {
	return func() tea.Msg {
		wfs, err := workflow.ListWorkflows()
		return workflowsLoadedMsg{workflows: wfs, err: err}
	}
}

// runWorkflowCmd runs a workflow asynchronously.
func runWorkflowCmd(wf *workflow.Workflow) tea.Cmd {
	return func() tea.Msg {
		dir, _ := os.Getwd()
		run, err := workflow.RunWorkflow(context.Background(), wf, dir, "")
		return workflowRunMsg{run: run, err: err}
	}
}

// updateWorkflows handles key events in the Workflows view.
func (m Model) updateWorkflows(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.workflowCursor < len(m.workflowList)-1 {
			m.workflowCursor++
		}
		return m, nil
	case "k", "up":
		if m.workflowCursor > 0 {
			m.workflowCursor--
		}
		return m, nil
	case "enter":
		if m.workflowCursor < len(m.workflowList) {
			wf := m.workflowList[m.workflowCursor]
			m.statusMsg = fmt.Sprintf("running %s...", wf.Name)
			return m, runWorkflowCmd(&wf)
		}
	case "d":
		if m.workflowCursor < len(m.workflowList) {
			wf := m.workflowList[m.workflowCursor]
			if wf.BuiltIn {
				m.err = "cannot delete built-in workflow"
				return m, nil
			}
			name := wf.Name
			return m, func() tea.Msg {
				workflow.DeleteWorkflow(name)
				wfs, err := workflow.ListWorkflows()
				return workflowsLoadedMsg{workflows: wfs, err: err}
			}
		}
	case "r":
		return m, loadWorkflowsCmd()
	}
	return m, nil
}

// renderWorkflowsView renders the workflows panel.
func renderWorkflowsView(workflows []workflow.Workflow, run *workflow.WorkflowRun, cursor int, width, height int) string {
	var b strings.Builder

	// Left panel: workflow list
	leftWidth := width / 2
	rightWidth := width - leftWidth - 2

	var leftContent strings.Builder
	leftContent.WriteString(detailLabelStyle.Render("  Workflows"))
	leftContent.WriteString("\n\n")

	if len(workflows) == 0 {
		leftContent.WriteString(formHintStyle.Render("  No workflows found. Press 'r' to refresh."))
	}

	for i, wf := range workflows {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == cursor {
			prefix = "▸ "
			style = style.Foreground(primaryColor).Bold(true)
		}

		tag := ""
		if wf.BuiltIn {
			tag = formHintStyle.Render(" (built-in)")
		}
		leftContent.WriteString(style.Render(prefix+wf.Name) + tag + "\n")

		if wf.Description != "" {
			desc := wf.Description
			if len(desc) > leftWidth-6 {
				desc = desc[:leftWidth-9] + "..."
			}
			leftContent.WriteString(formHintStyle.Render("    "+desc) + "\n")
		}
	}

	// Right panel: detail of selected workflow or run results
	var rightContent strings.Builder

	if run != nil && run.Status != "" {
		// Show run results
		rightContent.WriteString(detailLabelStyle.Render("Run: "+run.Workflow.Name))
		rightContent.WriteString("\n")
		rightContent.WriteString(fmt.Sprintf("Status: %s\n\n", run.Status))

		for i, result := range run.Results {
			icon := statusOkStyle.Render("✓")
			if result.Error != "" {
				icon = statusErrorStyle.Render("✗")
			}
			rightContent.WriteString(fmt.Sprintf("%s Step %d: %s\n", icon, i+1, result.StepName))
			if result.Error != "" {
				rightContent.WriteString(statusErrorStyle.Render("  "+result.Error) + "\n")
			} else if result.Result != "" {
				// Show truncated result
				preview := result.Result
				if len(preview) > rightWidth*3 {
					preview = preview[:rightWidth*3] + "..."
				}
				rightContent.WriteString(formHintStyle.Render("  "+preview) + "\n")
			}
			if result.Cost > 0 {
				rightContent.WriteString(fmt.Sprintf("  Cost: $%.4f\n", result.Cost))
			}
			rightContent.WriteString("\n")
		}
	} else if cursor < len(workflows) {
		// Show selected workflow detail
		wf := workflows[cursor]
		rightContent.WriteString(detailLabelStyle.Render(wf.Name))
		rightContent.WriteString("\n")
		if wf.Description != "" {
			rightContent.WriteString(wf.Description + "\n")
		}
		rightContent.WriteString(fmt.Sprintf("\nSteps: %d\n\n", len(wf.Steps)))

		for i, step := range wf.Steps {
			rightContent.WriteString(fmt.Sprintf("  %d. %s\n", i+1, detailLabelStyle.Render(step.Name)))
			prompt := step.Prompt
			if len(prompt) > rightWidth-6 {
				prompt = prompt[:rightWidth-9] + "..."
			}
			rightContent.WriteString(formHintStyle.Render("     "+prompt) + "\n")
			if step.WaitForApproval {
				rightContent.WriteString(statusWarnStyle.Render("     ⏸ requires approval") + "\n")
			}
		}
	}

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(leftWidth).Render(leftContent.String()),
		lipgloss.NewStyle().Width(rightWidth).MarginLeft(2).Render(rightContent.String()),
	)

	b.WriteString(content)
	return b.String()
}
