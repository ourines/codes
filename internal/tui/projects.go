package tui

import (
	"fmt"
	"sort"
	"strings"

	"codes/internal/config"
	"codes/internal/session"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

// projectItem implements the list.Item interface for project entries.
type projectItem struct {
	info config.ProjectInfo
}

func (i projectItem) Title() string {
	return i.info.Name
}

func (i projectItem) Description() string {
	var parts []string
	parts = append(parts, i.info.Path)
	if i.info.Remote != "" {
		parts = append(parts, "⇅ "+i.info.Remote)
	}
	if i.info.GitBranch != "" {
		branch := "⎇ " + i.info.GitBranch
		if i.info.GitDirty {
			branch += "*"
		}
		parts = append(parts, branch)
	}
	if !i.info.Exists {
		parts = append(parts, "✗ missing")
	}
	return strings.Join(parts, "  ")
}

func (i projectItem) FilterValue() string {
	s := i.info.Name + " " + i.info.Path
	if i.info.Remote != "" {
		s += " " + i.info.Remote
	}
	if i.info.GitBranch != "" {
		s += " " + i.info.GitBranch
	}
	return s
}

// loadProjects returns a sorted slice of list.Item from the configured projects.
func loadProjects() []list.Item {
	projects, err := config.ListProjects()
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]list.Item, 0, len(names))
	for _, name := range names {
		info := config.GetProjectInfoFromEntry(name, projects[name])
		items = append(items, projectItem{info: info})
	}

	return items
}

// renderProjectDetail renders the right-side detail panel for a project.
// When focused is true, sessions become selectable with a cursor at sessionCursor.
func renderProjectDetail(info config.ProjectInfo, width, height int, mgr *session.Manager, focused bool, sessionCursor int) string {
	var b strings.Builder

	if !info.Exists {
		b.WriteString(statusWarnStyle.Render("  Directory not found"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  %s %s",
			detailLabelStyle.Render("Path:"),
			detailValueStyle.Render(info.Path)))
		return detailBorderStyle.
			Width(width - 4).
			Height(height - 4).
			Render(b.String())
	}

	// Sessions
	if mgr != nil {
		runningSessions := mgr.GetRunningByProject(info.Name)

		if len(runningSessions) > 0 {
			b.WriteString(fmt.Sprintf("  %s %s\n\n",
				detailLabelStyle.Render("Sessions:"),
				statusOkStyle.Render(fmt.Sprintf("%d running", len(runningSessions)))))

			for i, s := range runningSessions {
				prefix := "    "
				if focused && i == sessionCursor {
					prefix = statusOkStyle.Render("  > ")
				}

				labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).
					Background(primaryColor).Padding(0, 1)
				if focused && i == sessionCursor {
					labelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).
						Background(secondaryColor).Padding(0, 1)
				}
				label := labelStyle.Render(s.ID)
				b.WriteString(fmt.Sprintf("%s%s\n", prefix, label))
				b.WriteString(fmt.Sprintf("%s%s PID %d  %s %s\n\n",
					prefix,
					statusOkStyle.Render("●"),
					s.PID,
					lipgloss.NewStyle().Foreground(mutedColor).Render("uptime"),
					detailValueStyle.Render(s.Uptime().String())))
			}

			// "+ New Session" option
			newIdx := len(runningSessions)
			prefix := "    "
			if focused && sessionCursor == newIdx {
				prefix = statusOkStyle.Render("  > ")
			}
			newStyle := lipgloss.NewStyle().Foreground(mutedColor)
			if focused && sessionCursor == newIdx {
				newStyle = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor)
			}
			b.WriteString(prefix + newStyle.Render("+ New Session") + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s %s\n",
				detailLabelStyle.Render("Sessions:"),
				lipgloss.NewStyle().Foreground(mutedColor).Render("No active sessions")))
		}
		b.WriteString("\n")
	}

	// Remote
	if info.Remote != "" {
		remoteLabel := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).
			Background(secondaryColor).Padding(0, 1).Render(info.Remote)
		b.WriteString(fmt.Sprintf("  %s %s\n",
			detailLabelStyle.Render("Remote:"),
			remoteLabel))
	}

	// Path
	b.WriteString(fmt.Sprintf("  %s %s",
		detailLabelStyle.Render("Path:"),
		detailValueStyle.Render(info.Path)))
	b.WriteString("\n")

	// Git branch + dirty indicator
	if info.GitBranch != "" {
		gitStatus := statusOkStyle.Render("✓ clean")
		if info.GitDirty {
			gitStatus = statusErrorStyle.Render("✗ dirty")
		}
		b.WriteString(fmt.Sprintf("  %s %s %s",
			detailLabelStyle.Render("Git:"),
			detailValueStyle.Render(info.GitBranch),
			gitStatus))
	} else {
		b.WriteString(fmt.Sprintf("  %s %s",
			detailLabelStyle.Render("Git:"),
			lipgloss.NewStyle().Foreground(mutedColor).Render("not a git repo")))
	}
	b.WriteString("\n")

	// CLAUDE.md presence
	claudeStatus := statusErrorStyle.Render("✗")
	if info.HasClaudeMD {
		claudeStatus = statusOkStyle.Render("CLAUDE.md ✓")
	}
	b.WriteString(fmt.Sprintf("  %s %s",
		detailLabelStyle.Render("Claude:"),
		claudeStatus))
	b.WriteString("\n")

	// Recent branches
	if len(info.RecentBranches) > 0 {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  %s", detailLabelStyle.Render("Branches:")))
		b.WriteString("\n")
		for _, branch := range info.RecentBranches {
			b.WriteString(fmt.Sprintf("    %s",
				detailValueStyle.Render(branch)))
			b.WriteString("\n")
		}
	}

	// Linked projects
	if len(info.Links) > 0 {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  %s", detailLabelStyle.Render("Links:")))
		b.WriteString("\n")
		for _, link := range info.Links {
			linkText := link.Name
			if link.Role != "" {
				linkText += lipgloss.NewStyle().Foreground(mutedColor).Render(" ("+link.Role+")")
			}
			b.WriteString(fmt.Sprintf("    %s %s",
				statusOkStyle.Render("→"),
				detailValueStyle.Render(linkText)))
			b.WriteString("\n")
		}
	}

	// Keybinding hints
	b.WriteString("\n")
	if focused {
		b.WriteString(formHintStyle.Render("  ↑↓: select  Enter: open  k: kill  ←: back"))
	} else {
		b.WriteString(formHintStyle.Render("  →: select session  Enter: new  l: links  k: kill"))
	}

	// Use highlighted border when focused
	borderStyle := detailBorderStyle
	if focused {
		borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(1, 2)
	}

	return borderStyle.
		Width(width - 4).
		Height(height - 4).
		Render(b.String())
}
