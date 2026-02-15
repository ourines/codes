package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codes/internal/agent"
)

// taskQueueLoadedMsg is sent after loading task queue data.
type taskQueueLoadedMsg struct {
	teams []string
	tasks []agent.Task
	err   error
}

// loadTaskQueueCmd loads tasks from all teams.
func loadTaskQueueCmd() tea.Cmd {
	return func() tea.Msg {
		teams, err := agent.ListTeams()
		if err != nil {
			return taskQueueLoadedMsg{err: err}
		}

		var allTasks []agent.Task
		for _, team := range teams {
			tasks, err := agent.ListTasks(team, "", "")
			if err != nil {
				continue
			}
			for _, t := range tasks {
				if t != nil {
					allTasks = append(allTasks, *t)
				}
			}
		}

		return taskQueueLoadedMsg{teams: teams, tasks: allTasks}
	}
}

// updateTaskQueue handles key events in the Task Queue view.
func (m Model) updateTaskQueue(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		m.taskQueueLoading = true
		return m, loadTaskQueueCmd()
	case "j", "down":
		maxIdx := len(m.taskQueueTasks) - 1
		if maxIdx < 0 {
			maxIdx = 0
		}
		if m.taskQueueCursor < maxIdx {
			m.taskQueueCursor++
		}
		return m, nil
	case "k", "up":
		if m.taskQueueCursor > 0 {
			m.taskQueueCursor--
		}
		return m, nil
	}
	return m, nil
}

// renderTaskQueueView renders the Task Queue panel.
func renderTaskQueueView(teams []string, tasks []agent.Task, loading bool, cursor int, width, height int) string {
	if loading {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(statsDimStyle.Render("Loading tasks..."))
	}

	if len(teams) == 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(statsDimStyle.Render("No teams configured. Use 'codes agent team create' to get started."))
	}

	var b strings.Builder

	// Header
	b.WriteString(statsHeaderStyle.Render(fmt.Sprintf("  Task Queue — %d team(s), %d task(s)", len(teams), len(tasks))))
	b.WriteString("\n\n")

	if len(tasks) == 0 {
		b.WriteString(statsDimStyle.Render("  No tasks. Press 'n' to create one."))
		return b.String()
	}

	// Separate into groups
	var queued, running, completed []agent.Task
	for _, t := range tasks {
		switch t.Status {
		case agent.TaskPending, agent.TaskAssigned:
			queued = append(queued, t)
		case agent.TaskRunning:
			running = append(running, t)
		default: // completed, failed, cancelled
			completed = append(completed, t)
		}
	}

	lineIdx := 0

	// Running
	if len(running) > 0 {
		b.WriteString(statsAccentStyle.Render(fmt.Sprintf("  ▶ Running (%d)", len(running))))
		b.WriteString("\n")
		for _, t := range running {
			prefix := "  "
			if lineIdx == cursor {
				prefix = "▸ "
			}
			owner := ""
			if t.Owner != "" {
				owner = fmt.Sprintf(" → %s", t.Owner)
			}
			b.WriteString(fmt.Sprintf("  %s#%-4d %s%s\n", prefix, t.ID, t.Subject, statsDimStyle.Render(owner)))
			lineIdx++
		}
		b.WriteString("\n")
	}

	// Queued
	if len(queued) > 0 {
		b.WriteString(statsHeaderStyle.Render(fmt.Sprintf("  ◆ Queued (%d)", len(queued))))
		b.WriteString("\n")
		for _, t := range queued {
			prefix := "  "
			if lineIdx == cursor {
				prefix = "▸ "
			}
			owner := ""
			if t.Owner != "" {
				owner = fmt.Sprintf(" → %s", t.Owner)
			}
			status := string(t.Status)
			b.WriteString(fmt.Sprintf("  %s#%-4d [%s] %s%s\n", prefix, t.ID, status, t.Subject, statsDimStyle.Render(owner)))
			lineIdx++
		}
		b.WriteString("\n")
	}

	// Completed (last 10)
	if len(completed) > 0 {
		shown := completed
		if len(shown) > 10 {
			shown = shown[len(shown)-10:]
		}
		b.WriteString(statsDimStyle.Render(fmt.Sprintf("  ✓ Completed (%d)", len(completed))))
		b.WriteString("\n")
		for _, t := range shown {
			prefix := "  "
			if lineIdx == cursor {
				prefix = "▸ "
			}
			statusIcon := "✓"
			if t.Status == agent.TaskFailed {
				statusIcon = "✗"
			} else if t.Status == agent.TaskCancelled {
				statusIcon = "○"
			}
			b.WriteString(fmt.Sprintf("  %s%s #%-4d %s\n", prefix, statusIcon, t.ID, statsDimStyle.Render(t.Subject)))
			lineIdx++
		}
	}

	return b.String()
}
