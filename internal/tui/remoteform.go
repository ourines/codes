package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codes/internal/config"
)

// remoteAddedMsg is sent when a new remote host is submitted.
type remoteAddedMsg struct {
	host config.RemoteHost
}

// remoteFormModel is the model for the add-remote form.
type remoteFormModel struct {
	nameInput     textinput.Model
	hostInput     textinput.Model
	userInput     textinput.Model
	portInput     textinput.Model
	identityInput textinput.Model
	focused       int // 0=name, 1=host, 2=user, 3=port, 4=identity
	err           string
}

func newRemoteForm() remoteFormModel {
	ni := textinput.New()
	ni.Placeholder = "my-server"
	ni.CharLimit = 50
	ni.Focus()

	hi := textinput.New()
	hi.Placeholder = "192.168.1.100 or hostname"
	hi.CharLimit = 200

	ui := textinput.New()
	ui.Placeholder = "root (optional)"
	ui.CharLimit = 50

	pi := textinput.New()
	pi.Placeholder = "22 (optional)"
	pi.CharLimit = 5

	ii := textinput.New()
	ii.Placeholder = "~/.ssh/id_rsa (optional)"
	ii.CharLimit = 200

	return remoteFormModel{
		nameInput:     ni,
		hostInput:     hi,
		userInput:     ui,
		portInput:     pi,
		identityInput: ii,
		focused:       0,
	}
}

func (m *remoteFormModel) focusRemoteInput() {
	m.nameInput.Blur()
	m.hostInput.Blur()
	m.userInput.Blur()
	m.portInput.Blur()
	m.identityInput.Blur()
	switch m.focused {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.hostInput.Focus()
	case 2:
		m.userInput.Focus()
	case 3:
		m.portInput.Focus()
	case 4:
		m.identityInput.Focus()
	}
}

func (m remoteFormModel) Update(msg tea.Msg) (remoteFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focused = (m.focused + 1) % 5
			m.focusRemoteInput()
			return m, nil
		case "shift+tab", "up":
			m.focused = (m.focused + 4) % 5
			m.focusRemoteInput()
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.nameInput.Value())
			host := strings.TrimSpace(m.hostInput.Value())
			user := strings.TrimSpace(m.userInput.Value())
			portStr := strings.TrimSpace(m.portInput.Value())
			identity := strings.TrimSpace(m.identityInput.Value())

			if name == "" {
				m.err = "Name is required"
				return m, nil
			}
			if host == "" {
				m.err = "Host is required"
				return m, nil
			}

			var port int
			if portStr != "" {
				for _, c := range portStr {
					if c < '0' || c > '9' {
						m.err = "Port must be a number"
						return m, nil
					}
				}
				for _, c := range portStr {
					port = port*10 + int(c-'0')
				}
			}

			m.err = ""
			rh := config.RemoteHost{
				Name:     name,
				Host:     host,
				User:     user,
				Port:     port,
				Identity: identity,
			}
			return m, func() tea.Msg {
				return remoteAddedMsg{host: rh}
			}
		}
	}

	// Update focused text input
	var cmd tea.Cmd
	switch m.focused {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		m.hostInput, cmd = m.hostInput.Update(msg)
	case 2:
		m.userInput, cmd = m.userInput.Update(msg)
	case 3:
		m.portInput, cmd = m.portInput.Update(msg)
	case 4:
		m.identityInput, cmd = m.identityInput.Update(msg)
	}
	return m, cmd
}

func (m remoteFormModel) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		MarginBottom(1).
		Render("Add Remote Host")

	b.WriteString(title + "\n\n")

	fields := []struct {
		label string
		input textinput.Model
		idx   int
	}{
		{"Name", m.nameInput, 0},
		{"Host", m.hostInput, 1},
		{"User", m.userInput, 2},
		{"Port", m.portInput, 3},
		{"Identity", m.identityInput, 4},
	}

	for _, f := range fields {
		label := formLabelStyle.Render(f.label)
		if m.focused == f.idx {
			label = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("▸ " + f.label)
		}
		b.WriteString(label + "\n")
		b.WriteString(f.input.View() + "\n\n")
	}

	if m.err != "" {
		b.WriteString(statusErrorStyle.Render("⚠ "+m.err) + "\n\n")
	}

	return b.String()
}
