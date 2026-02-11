package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// projectAddedMsg is sent when the user submits the add form.
type projectAddedMsg struct {
	name string
	path string
}

// addFormModel is the model for the add-project form.
type addFormModel struct {
	nameInput textinput.Model
	pathInput textinput.Model
	focused   int // 0=name, 1=path
	err       string
}

// newAddForm creates a new add-project form with two text inputs.
func newAddForm() addFormModel {
	ni := textinput.New()
	ni.Placeholder = "project-name"
	ni.CharLimit = 50
	ni.Focus()

	pi := textinput.New()
	pi.Placeholder = "/path/to/project"
	pi.CharLimit = 200

	return addFormModel{
		nameInput: ni,
		pathInput: pi,
		focused:   0,
	}
}

// focusInput updates Focus/Blur state on the inputs based on m.focused.
func (m *addFormModel) focusInput() {
	if m.focused == 0 {
		m.nameInput.Focus()
		m.pathInput.Blur()
	} else {
		m.nameInput.Blur()
		m.pathInput.Focus()
	}
}

// Update handles input for the add form.
func (m addFormModel) Update(msg tea.Msg) (addFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focused = (m.focused + 1) % 2
			m.focusInput()
			return m, nil
		case "shift+tab", "up":
			m.focused = (m.focused + 1) % 2
			m.focusInput()
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.nameInput.Value())
			path := strings.TrimSpace(m.pathInput.Value())
			if name == "" && path == "" {
				m.err = "Name and path are required"
				return m, nil
			}
			if name == "" {
				m.err = "Name is required"
				return m, nil
			}
			if path == "" {
				m.err = "Path is required"
				return m, nil
			}
			m.err = ""
			return m, func() tea.Msg {
				return projectAddedMsg{name: name, path: path}
			}
		}
	}

	// Update the focused input.
	var cmd tea.Cmd
	if m.focused == 0 {
		m.nameInput, cmd = m.nameInput.Update(msg)
	} else {
		m.pathInput, cmd = m.pathInput.Update(msg)
	}
	return m, cmd
}

// View renders the add form.
func (m addFormModel) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		MarginBottom(1).
		Render("Add Project")

	b.WriteString(title + "\n\n")
	b.WriteString(formLabelStyle.Render("Name") + "\n")
	b.WriteString(m.nameInput.View() + "\n\n")
	b.WriteString(formLabelStyle.Render("Path") + "\n")
	b.WriteString(m.pathInput.View() + "\n\n")

	if m.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(dangerColor)
		b.WriteString(errStyle.Render(m.err) + "\n\n")
	}

	b.WriteString(formHintStyle.Render("Tab to switch fields, Enter to add, Esc to cancel"))

	return b.String()
}
