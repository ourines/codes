package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codes/internal/session"
)

// rollbackItem represents a file in the partial rollback selection list.
type rollbackItem struct {
	file     session.DiffFile
	selected bool // true = rollback, false = keep
}

// Messages for checkpoint flow.

type checkpointCreatedMsg struct {
	cp  *session.Checkpoint
	err error
}

type diffLoadedMsg struct {
	summary *session.DiffSummary
	err     error
}

type rollbackDoneMsg struct {
	err error
}

// Checkpoint-specific styles.
var (
	cpFileAddStyle = lipgloss.NewStyle().Foreground(secondaryColor)
	cpFileDelStyle = lipgloss.NewStyle().Foreground(dangerColor)
	cpFileModStyle = lipgloss.NewStyle().Foreground(warnColor)
	cpSelectedStyle = lipgloss.NewStyle().Foreground(dangerColor).Bold(true)
	cpKeptStyle     = lipgloss.NewStyle().Foreground(secondaryColor)
)

// updateSessionSummary handles key events in the session summary view.
func (m Model) updateSessionSummary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "enter":
		// Keep changes and return to projects
		m.state = viewProjects
		m.checkpoint = nil
		m.diffSummary = nil
		return m, nil
	case "r":
		// Rollback all
		if m.checkpoint != nil {
			cp := m.checkpoint
			return m, func() tea.Msg {
				err := session.RollbackAll(cp.Dir, cp)
				return rollbackDoneMsg{err: err}
			}
		}
	case "p":
		// Enter partial rollback
		if m.diffSummary != nil && len(m.diffSummary.Files) > 0 {
			m.rollbackItems = make([]rollbackItem, len(m.diffSummary.Files))
			for i, f := range m.diffSummary.Files {
				m.rollbackItems[i] = rollbackItem{file: f, selected: false}
			}
			m.rollbackCursor = 0
			m.state = viewPartialRollback
		}
		return m, nil
	case "esc":
		m.state = viewProjects
		m.checkpoint = nil
		m.diffSummary = nil
		return m, nil
	}
	return m, nil
}

// updatePartialRollback handles key events in the partial rollback view.
func (m Model) updatePartialRollback(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.state = viewSessionSummary
		return m, nil
	case "up", "k":
		if m.rollbackCursor > 0 {
			m.rollbackCursor--
		}
		return m, nil
	case "down", "j":
		if m.rollbackCursor < len(m.rollbackItems)-1 {
			m.rollbackCursor++
		}
		return m, nil
	case " ":
		// Toggle selection
		if m.rollbackCursor < len(m.rollbackItems) {
			m.rollbackItems[m.rollbackCursor].selected = !m.rollbackItems[m.rollbackCursor].selected
		}
		return m, nil
	case "enter":
		// Apply partial rollback
		var filesToRollback []string
		for _, item := range m.rollbackItems {
			if item.selected {
				filesToRollback = append(filesToRollback, item.file.Path)
			}
		}
		if len(filesToRollback) > 0 && m.checkpoint != nil {
			cp := m.checkpoint
			return m, func() tea.Msg {
				err := session.RollbackFiles(cp.Dir, cp, filesToRollback)
				return rollbackDoneMsg{err: err}
			}
		}
		// Nothing selected, go back
		m.state = viewSessionSummary
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// renderSessionSummary renders the session summary with diff info.
func (m Model) renderSessionSummary(width, height int) string {
	var b strings.Builder

	b.WriteString(statsHeaderStyle.Render("  Session Summary"))
	b.WriteString("\n\n")

	if m.diffSummary == nil || len(m.diffSummary.Files) == 0 {
		b.WriteString(statsDimStyle.Render("  No changes detected."))
		b.WriteString("\n\n")
		b.WriteString(formHintStyle.Render("  enter: return to projects  esc: cancel"))
		return b.String()
	}

	// Total stats
	total := fmt.Sprintf("  %d file(s) changed, %s, %s",
		len(m.diffSummary.Files),
		cpFileAddStyle.Render(fmt.Sprintf("+%d", m.diffSummary.TotalAdded)),
		cpFileDelStyle.Render(fmt.Sprintf("-%d", m.diffSummary.TotalDel)))
	b.WriteString(total)
	b.WriteString("\n\n")

	// File list
	maxFiles := height - 8
	if maxFiles < 3 {
		maxFiles = 3
	}
	files := m.diffSummary.Files
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}

	for _, f := range files {
		statusIcon := fileStatusIcon(f.Status)
		adds := ""
		dels := ""
		if f.Additions > 0 {
			adds = cpFileAddStyle.Render(fmt.Sprintf("+%d", f.Additions))
		}
		if f.Deletions > 0 {
			dels = cpFileDelStyle.Render(fmt.Sprintf("-%d", f.Deletions))
		}

		stats := strings.TrimSpace(adds + " " + dels)
		if stats != "" {
			stats = "  " + stats
		}

		line := fmt.Sprintf("  %s %s%s", statusIcon, f.Path, stats)
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(m.diffSummary.Files) > maxFiles {
		b.WriteString(statsDimStyle.Render(fmt.Sprintf("  ... and %d more files", len(m.diffSummary.Files)-maxFiles)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(formHintStyle.Render("  r: rollback all  p: partial rollback  enter: keep & return  esc: cancel"))

	return b.String()
}

// renderPartialRollback renders the partial rollback file selection view.
func (m Model) renderPartialRollback(width, height int) string {
	var b strings.Builder

	b.WriteString(statsHeaderStyle.Render("  Partial Rollback"))
	b.WriteString("  ")
	b.WriteString(statsDimStyle.Render("(space: toggle  enter: apply  esc: back)"))
	b.WriteString("\n\n")

	selectedCount := 0
	for _, item := range m.rollbackItems {
		if item.selected {
			selectedCount++
		}
	}

	b.WriteString(fmt.Sprintf("  %d file(s) selected for rollback\n\n",
		selectedCount))

	maxFiles := height - 8
	if maxFiles < 3 {
		maxFiles = 3
	}

	// Calculate scroll window
	startIdx := 0
	if m.rollbackCursor >= maxFiles {
		startIdx = m.rollbackCursor - maxFiles + 1
	}
	endIdx := startIdx + maxFiles
	if endIdx > len(m.rollbackItems) {
		endIdx = len(m.rollbackItems)
	}

	for i := startIdx; i < endIdx; i++ {
		item := m.rollbackItems[i]
		cursor := "  "
		if i == m.rollbackCursor {
			cursor = "> "
		}

		checkbox := "[ ]"
		checkStyle := cpKeptStyle
		if item.selected {
			checkbox = "[x]"
			checkStyle = cpSelectedStyle
		}

		statusIcon := fileStatusIcon(item.file.Status)
		line := fmt.Sprintf("%s%s %s %s",
			cursor,
			checkStyle.Render(checkbox),
			statusIcon,
			item.file.Path)

		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(m.rollbackItems) > maxFiles {
		b.WriteString(statsDimStyle.Render(fmt.Sprintf("\n  showing %d-%d of %d files",
			startIdx+1, endIdx, len(m.rollbackItems))))
		b.WriteString("\n")
	}

	return b.String()
}

// fileStatusIcon returns a styled status indicator for a diff file.
func fileStatusIcon(status string) string {
	switch status {
	case "A":
		return cpFileAddStyle.Render("A")
	case "D":
		return cpFileDelStyle.Render("D")
	case "R":
		return cpFileModStyle.Render("R")
	default:
		return cpFileModStyle.Render("M")
	}
}
