package tui

import (
	"fmt"
	"sort"
	"strings"

	"codes/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// projectLinkedMsg is sent when a link is created.
type projectLinkedMsg struct {
	projectName  string
	linkedName   string
	role         string
	err          error
}

// projectUnlinkedMsg is sent when a link is removed.
type projectUnlinkedMsg struct {
	projectName string
	linkedName  string
	err         error
}

// linkFormModel is the model for managing project links.
type linkFormModel struct {
	projectName       string               // source project name
	existingLinks     []config.ProjectLink // existing links
	availableProjects []string             // projects that can be linked
	roleInput         textinput.Model
	mode              string // "list" | "add" | "unlink"
	cursor            int    // list cursor
	err               string
}

// newLinkForm creates a new link form for a project.
func newLinkForm(projectName string, cfg *config.Config) linkFormModel {
	ri := textinput.New()
	ri.Placeholder = "e.g. API provider, deployment target"
	ri.CharLimit = 100

	// Get existing links
	var existingLinks []config.ProjectLink
	if entry, exists := cfg.Projects[projectName]; exists {
		existingLinks = entry.Links
	}

	// Get available projects (all except self and already linked)
	linkedMap := make(map[string]bool)
	for _, link := range existingLinks {
		linkedMap[link.Name] = true
	}

	var available []string
	for name := range cfg.Projects {
		if name != projectName && !linkedMap[name] {
			available = append(available, name)
		}
	}
	sort.Strings(available)

	return linkFormModel{
		projectName:       projectName,
		existingLinks:     existingLinks,
		availableProjects: available,
		roleInput:         ri,
		mode:              "list",
		cursor:            0,
	}
}

// updateLinkForm handles updates for the link form.
func (m Model) updateLinkForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lf := &m.linkForm

	switch lf.mode {
	case "list":
		switch msg.String() {
		case "esc", "q":
			m.state = viewProjects
			m.focus = focusLeft
			return m, nil

		case "a":
			if len(lf.availableProjects) > 0 {
				lf.mode = "add"
				lf.cursor = 0
				lf.roleInput.Reset()
			}
			return m, nil

		case "d", "x":
			if len(lf.existingLinks) > 0 && lf.cursor < len(lf.existingLinks) {
				lf.mode = "unlink"
			}
			return m, nil

		case "up", "k":
			if lf.cursor > 0 {
				lf.cursor--
			}
			return m, nil

		case "down", "j":
			if lf.cursor < len(lf.existingLinks)-1 {
				lf.cursor++
			}
			return m, nil
		}

	case "add":
		switch msg.String() {
		case "esc":
			lf.mode = "list"
			lf.roleInput.Blur()
			return m, nil

		case "enter":
			if lf.cursor < len(lf.availableProjects) {
				linkedName := lf.availableProjects[lf.cursor]
				role := strings.TrimSpace(lf.roleInput.Value())

				cmd := func() tea.Msg {
					err := config.LinkProject(lf.projectName, linkedName, role)
					return projectLinkedMsg{
						projectName: lf.projectName,
						linkedName:  linkedName,
						role:        role,
						err:         err,
					}
				}
				return m, cmd
			}
			return m, nil

		case "up", "k":
			if lf.cursor > 0 {
				lf.cursor--
			}
			return m, nil

		case "down", "j":
			if lf.cursor < len(lf.availableProjects)-1 {
				lf.cursor++
			}
			return m, nil

		case "tab":
			lf.roleInput.Focus()
			return m, nil

		default:
			if lf.roleInput.Focused() {
				var cmd tea.Cmd
				lf.roleInput, cmd = lf.roleInput.Update(msg)
				return m, cmd
			}
		}

	case "unlink":
		switch msg.String() {
		case "y", "Y":
			if lf.cursor < len(lf.existingLinks) {
				linkedName := lf.existingLinks[lf.cursor].Name

				cmd := func() tea.Msg {
					err := config.UnlinkProject(lf.projectName, linkedName)
					return projectUnlinkedMsg{
						projectName: lf.projectName,
						linkedName:  linkedName,
						err:         err,
					}
				}
				return m, cmd
			}
			return m, nil

		case "n", "N", "esc":
			lf.mode = "list"
			return m, nil
		}
	}

	return m, nil
}

// viewLinkForm renders the link form.
func (m Model) viewLinkForm() string {
	lf := m.linkForm

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Manage Links: %s", lf.projectName)))
	b.WriteString("\n\n")

	switch lf.mode {
	case "list":
		// Existing links section
		if len(lf.existingLinks) > 0 {
			b.WriteString(detailLabelStyle.Render("Linked Projects:"))
			b.WriteString("\n\n")

			for i, link := range lf.existingLinks {
				prefix := "  "
				if i == lf.cursor {
					prefix = statusOkStyle.Render("> ")
				}

				linkText := link.Name
				if link.Role != "" {
					linkText += lipgloss.NewStyle().Foreground(mutedColor).Render(" (" + link.Role + ")")
				}

				b.WriteString(prefix + linkText + "\n")
			}
			b.WriteString("\n")
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Render("  No linked projects"))
			b.WriteString("\n\n")
		}

		// Available projects count
		if len(lf.availableProjects) > 0 {
			b.WriteString(formHintStyle.Render(fmt.Sprintf("  %d projects available to link", len(lf.availableProjects))))
		} else {
			b.WriteString(formHintStyle.Render("  No more projects available to link"))
		}
		b.WriteString("\n\n")

		// Help
		b.WriteString(formHintStyle.Render("  a: add link  d: remove link  esc: back"))

	case "add":
		b.WriteString(detailLabelStyle.Render("Select Project to Link:"))
		b.WriteString("\n\n")

		for i, name := range lf.availableProjects {
			prefix := "  "
			style := lipgloss.NewStyle()
			if i == lf.cursor {
				prefix = statusOkStyle.Render("> ")
				style = style.Bold(true).Foreground(secondaryColor)
			}
			b.WriteString(prefix + style.Render(name) + "\n")
		}

		b.WriteString("\n")
		b.WriteString(detailLabelStyle.Render("Role (optional):"))
		b.WriteString("\n")
		b.WriteString("  " + lf.roleInput.View())
		b.WriteString("\n\n")

		// Help
		b.WriteString(formHintStyle.Render("  ↑↓: select  tab: edit role  Enter: confirm  esc: cancel"))

	case "unlink":
		if lf.cursor < len(lf.existingLinks) {
			linkedName := lf.existingLinks[lf.cursor].Name
			b.WriteString(statusWarnStyle.Render(fmt.Sprintf("Unlink %s?", linkedName)))
			b.WriteString("\n\n")
			b.WriteString(formHintStyle.Render("  y: yes  n: no"))
		}
	}

	if lf.err != "" {
		b.WriteString("\n\n")
		b.WriteString(statusErrorStyle.Render(lf.err))
	}

	innerWidth := m.width - 4
	innerHeight := m.height - 2

	return appStyle.Render(
		lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			Width(innerWidth).
			Height(innerHeight).
			Render(b.String()),
	)
}
