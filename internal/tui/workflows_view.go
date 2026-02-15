package tui

import (
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
	run *workflow.WorkflowRunResult
	err error
}

// loadWorkflowsCmd loads all workflows asynchronously.
func loadWorkflowsCmd() tea.Cmd {
	return func() tea.Msg {
		wfs, err := workflow.ListWorkflows()
		return workflowsLoadedMsg{workflows: wfs, err: err}
	}
}

// runWorkflowCmd launches a workflow as an agent team (non-blocking).
func runWorkflowCmd(wf *workflow.Workflow) tea.Cmd {
	return func() tea.Msg {
		dir, _ := os.Getwd()
		result, err := workflow.RunWorkflow(wf, workflow.RunWorkflowOptions{
			WorkDir: dir,
		})
		return workflowRunMsg{run: result, err: err}
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
			m.statusMsg = fmt.Sprintf("launching %s...", wf.Name)
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
func renderWorkflowsView(workflows []workflow.Workflow, run *workflow.WorkflowRunResult, cursor int, width, height int) string {
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

	if run != nil {
		// Show run results (team launch info)
		rightContent.WriteString(detailLabelStyle.Render("Launched"))
		rightContent.WriteString("\n\n")
		rightContent.WriteString(fmt.Sprintf("Team: %s\n", run.TeamName))
		rightContent.WriteString(fmt.Sprintf("Agents started: %d\n", run.Agents))
		rightContent.WriteString(fmt.Sprintf("Tasks created:  %d\n", run.Tasks))
		rightContent.WriteString("\n")
		rightContent.WriteString(formHintStyle.Render("Use 'codes agent status "+run.TeamName+"' to monitor") + "\n")
	} else if cursor < len(workflows) {
		// Show selected workflow detail
		wf := workflows[cursor]
		rightContent.WriteString(detailLabelStyle.Render(wf.Name))
		rightContent.WriteString("\n")
		if wf.Description != "" {
			rightContent.WriteString(wf.Description + "\n")
		}
		rightContent.WriteString(fmt.Sprintf("\nAgents: %d  Tasks: %d\n\n", len(wf.Agents), len(wf.Tasks)))

		// Show agents
		if len(wf.Agents) > 0 {
			rightContent.WriteString(detailLabelStyle.Render("Agents:") + "\n")
			for _, a := range wf.Agents {
				rightContent.WriteString(fmt.Sprintf("  - %s", a.Name))
				if a.Role != "" {
					role := a.Role
					if len(role) > rightWidth-len(a.Name)-8 {
						role = role[:rightWidth-len(a.Name)-11] + "..."
					}
					rightContent.WriteString(formHintStyle.Render(" ("+role+")"))
				}
				rightContent.WriteString("\n")
			}
			rightContent.WriteString("\n")
		}

		// Show tasks
		if len(wf.Tasks) > 0 {
			rightContent.WriteString(detailLabelStyle.Render("Tasks:") + "\n")
			for i, t := range wf.Tasks {
				rightContent.WriteString(fmt.Sprintf("  %d. %s", i+1, t.Subject))
				if t.Assign != "" {
					rightContent.WriteString(formHintStyle.Render(" -> "+t.Assign))
				}
				rightContent.WriteString("\n")
				if len(t.BlockedBy) > 0 {
					rightContent.WriteString(statusWarnStyle.Render(fmt.Sprintf("     blocked by: %v", t.BlockedBy)) + "\n")
				}
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
