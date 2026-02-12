package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codes/internal/config"
)

// profileAddedMsg is sent when a new profile is submitted.
type profileAddedMsg struct {
	cfg    config.APIConfig
	tested bool   // whether API test passed
	status string // "active" or "inactive"
}

// profileTestResultMsg is sent after async API test completes.
type profileTestResultMsg struct {
	name   string
	active bool
}

type profileFormModel struct {
	nameInput  textinput.Model
	urlInput   textinput.Model
	tokenInput textinput.Model
	focused    int  // 0=name, 1=url, 2=token, 3=skip toggle
	skip       bool // skip permissions toggle
	err        string
	testing    bool // API test in progress
}

func newProfileForm() profileFormModel {
	ni := textinput.New()
	ni.Placeholder = "work"
	ni.CharLimit = 50
	ni.Focus()

	ui := textinput.New()
	ui.Placeholder = "https://api.anthropic.com"
	ui.CharLimit = 200

	ti := textinput.New()
	ti.Placeholder = "sk-ant-..."
	ti.CharLimit = 200
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

	return profileFormModel{
		nameInput:  ni,
		urlInput:   ui,
		tokenInput: ti,
		focused:    0,
	}
}

func (m *profileFormModel) focusProfileInput() {
	m.nameInput.Blur()
	m.urlInput.Blur()
	m.tokenInput.Blur()
	switch m.focused {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.urlInput.Focus()
	case 2:
		m.tokenInput.Focus()
	}
}

func (m profileFormModel) Update(msg tea.Msg) (profileFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focused = (m.focused + 1) % 4
			m.focusProfileInput()
			return m, nil
		case "shift+tab", "up":
			m.focused = (m.focused + 3) % 4 // -1 mod 4
			m.focusProfileInput()
			return m, nil
		case " ":
			if m.focused == 3 {
				m.skip = !m.skip
				return m, nil
			}
		case "enter":
			if m.testing {
				return m, nil
			}
			name := strings.TrimSpace(m.nameInput.Value())
			url := strings.TrimSpace(m.urlInput.Value())
			token := strings.TrimSpace(m.tokenInput.Value())

			if name == "" {
				m.err = "Name is required"
				return m, nil
			}
			if url == "" {
				m.err = "Base URL is required"
				return m, nil
			}
			if token == "" {
				m.err = "Auth Token is required"
				return m, nil
			}

			m.err = ""
			m.testing = true

			newCfg := config.APIConfig{
				Name: name,
				Env: map[string]string{
					"ANTHROPIC_BASE_URL":  url,
					"ANTHROPIC_AUTH_TOKEN": token,
				},
			}
			if m.skip {
				skip := true
				newCfg.SkipPermissions = &skip
			}

			// Test API connection in background
			return m, func() tea.Msg {
				active := config.TestAPIConfig(newCfg)
				status := "inactive"
				if active {
					status = "active"
				}
				newCfg.Status = status
				return profileAddedMsg{
					cfg:    newCfg,
					tested: true,
					status: status,
				}
			}
		}
	}

	// Update focused text input
	var cmd tea.Cmd
	switch m.focused {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		m.urlInput, cmd = m.urlInput.Update(msg)
	case 2:
		m.tokenInput, cmd = m.tokenInput.Update(msg)
	}
	return m, cmd
}

func (m profileFormModel) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		MarginBottom(1).
		Render("Add Profile")

	b.WriteString(title + "\n\n")

	// Name
	label := formLabelStyle.Render("Name")
	if m.focused == 0 {
		label = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("▸ Name")
	}
	b.WriteString(label + "\n")
	b.WriteString(m.nameInput.View() + "\n\n")

	// URL
	label = formLabelStyle.Render("Base URL")
	if m.focused == 1 {
		label = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("▸ Base URL")
	}
	b.WriteString(label + "\n")
	b.WriteString(m.urlInput.View() + "\n\n")

	// Token
	label = formLabelStyle.Render("Auth Token")
	if m.focused == 2 {
		label = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("▸ Auth Token")
	}
	b.WriteString(label + "\n")
	b.WriteString(m.tokenInput.View() + "\n\n")

	// Skip permissions toggle
	toggleLabel := formLabelStyle.Render("Skip Permissions")
	if m.focused == 3 {
		toggleLabel = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("▸ Skip Permissions")
	}
	toggle := "[ ] no"
	if m.skip {
		toggle = "[✓] yes"
	}
	b.WriteString(toggleLabel + "  " + detailValueStyle.Render(toggle) + "\n\n")

	if m.testing {
		b.WriteString(statusWarnStyle.Render("⏳ Testing API connection...") + "\n\n")
	}

	if m.err != "" {
		b.WriteString(statusErrorStyle.Render("⚠ "+m.err) + "\n\n")
	}

	return b.String()
}
