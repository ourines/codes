package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codes/internal/config"
	"codes/internal/session"
)

type viewState int

const (
	viewProjects viewState = iota
	viewProfiles
	viewSettings
	viewAddForm
	viewAddProfile
)

type panelFocus int

const (
	focusLeft  panelFocus = iota // project/profile list
	focusRight                   // detail panel (sessions)
)

// Model is the main TUI model.
type Model struct {
	state         viewState
	focus         panelFocus
	projectList   list.Model
	profileList   list.Model
	addForm       addFormModel
	profileForm   profileFormModel
	help          help.Model
	cfg           *config.Config
	width         int
	height        int
	err           string
	sessionMgr    *session.Manager
	sessionCursor int // cursor index within right-panel session list
	settings      settingsModel
}

// projectDeletedMsg is sent after deleting a project.
type projectDeletedMsg struct{ name string }

// profileSwitchedMsg is sent after switching the default profile.
type profileSwitchedMsg struct{ name string }

// sessionStartedMsg is sent after attempting to start a session.
type sessionStartedMsg struct {
	name string
	err  error
}

// sessionTickMsg triggers periodic session status refresh.
type sessionTickMsg struct{}

func sessionTick() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return sessionTickMsg{}
	})
}

// NewModel creates the initial TUI model.
func NewModel() Model {
	// Load projects
	projectItems := loadProjects()
	projectDelegate := list.NewDefaultDelegate()
	projectDelegate.ShowDescription = true
	pl := list.New(projectItems, projectDelegate, 0, 0)
	pl.Title = "Projects"
	pl.SetShowHelp(false)
	pl.SetShowStatusBar(true)
	pl.SetFilteringEnabled(true)

	// Load profiles
	profileItems, _ := loadProfiles()
	profileDelegate := list.NewDefaultDelegate()
	profileDelegate.ShowDescription = true
	cl := list.New(profileItems, profileDelegate, 0, 0)
	cl.Title = "Profiles"
	cl.SetShowHelp(false)
	cl.SetShowStatusBar(true)
	cl.SetFilteringEnabled(true)

	cfg, _ := config.LoadConfig()

	return Model{
		state:       viewProjects,
		projectList: pl,
		profileList: cl,
		help:        help.New(),
		cfg:         cfg,
		sessionMgr:  session.NewManager(config.GetTerminal()),
		settings:    newSettingsModel(cfg),
	}
}

func (m Model) Init() tea.Cmd {
	return sessionTick()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Account for appStyle padding: Padding(1, 2) = 2 vertical, 4 horizontal
		innerWidth := m.width - 4
		innerHeight := m.height - 2

		// Calculate list dimensions (left panel takes ~50% of inner width)
		listWidth := innerWidth / 2
		listHeight := innerHeight - 6 // leave room for header + footer + help
		m.projectList.SetSize(listWidth, listHeight)
		m.profileList.SetSize(listWidth, listHeight)
		m.help.Width = innerWidth
		return m, nil

	case tea.KeyMsg:
		// Global keys (not when filtering or in form)
		if m.state == viewAddForm {
			return m.updateAddForm(msg)
		}
		if m.state == viewAddProfile {
			return m.updateProfileForm(msg)
		}
		if m.state == viewSettings {
			return m.updateSettings(msg)
		}

		// Right panel focused — handle session selection
		if m.focus == focusRight && m.state == viewProjects {
			return m.updateRightPanel(msg)
		}

		// Don't intercept keys when the current list is filtering
		if m.state == viewProjects && m.projectList.FilterState() == list.Filtering {
			return m.updateList(msg)
		}
		if m.state == viewProfiles && m.profileList.FilterState() == list.Filtering {
			return m.updateList(msg)
		}

		switch {
		case msg.String() == "q" || msg.String() == "ctrl+c":
			return m, tea.Quit

		case msg.String() == "tab":
			switch m.state {
			case viewProjects:
				m.state = viewProfiles
			case viewProfiles:
				m.state = viewSettings
				m.settings = newSettingsModel(m.cfg)
			default:
				m.state = viewProjects
			}
			m.focus = focusLeft
			return m, nil

		case msg.String() == "right" || msg.String() == "l":
			if m.state == viewProjects && m.focus == focusLeft {
				// Only activate right panel if there are running sessions
				if item, ok := m.projectList.SelectedItem().(projectItem); ok {
					running := m.sessionMgr.GetRunningByProject(item.info.Name)
					if len(running) > 0 {
						m.focus = focusRight
						m.sessionCursor = 0
						return m, nil
					}
				}
			}

		case msg.String() == "a" && m.state == viewProjects:
			m.state = viewAddForm
			m.addForm = newAddForm()
			return m, nil

		case msg.String() == "a" && m.state == viewProfiles:
			m.state = viewAddProfile
			m.profileForm = newProfileForm()
			return m, nil

		case msg.String() == "d" && m.state == viewProjects:
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				return m, func() tea.Msg {
					config.RemoveProject(item.info.Name)
					return projectDeletedMsg{name: item.info.Name}
				}
			}

		case msg.String() == "k" && m.state == viewProjects:
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				m.sessionMgr.KillByProject(item.info.Name)
			}
			return m, nil

		case msg.String() == "t" && m.state == viewProjects:
			// Cycle terminal: terminal → iterm → warp → terminal
			options := config.TerminalOptions()
			current := config.GetTerminal()
			if current == "" {
				current = "terminal"
			}
			next := options[0]
			for i, opt := range options {
				if opt == current && i+1 < len(options) {
					next = options[i+1]
					break
				}
			}
			config.SetTerminal(next)
			m.sessionMgr = session.NewManager(next)
			return m, nil

		case msg.String() == "enter":
			if m.state == viewProjects {
				if item, ok := m.projectList.SelectedItem().(projectItem); ok {
					if !item.info.Exists {
						return m, nil
					}
					name := item.info.Name
					path := item.info.Path

					// Start a new session
					args, env := config.ClaudeCmdSpec()
					return m, func() tea.Msg {
						_, err := m.sessionMgr.StartSession(name, path, args, env)
						return sessionStartedMsg{name: name, err: err}
					}
				}
			} else if m.state == viewProfiles {
				if item, ok := m.profileList.SelectedItem().(profileItem); ok {
					return m, func() tea.Msg {
						if m.cfg != nil {
							m.cfg.Default = item.cfg.Name
							config.SaveConfig(m.cfg)
						}
						return profileSwitchedMsg{name: item.cfg.Name}
					}
				}
			}
		}

	case sessionStartedMsg:
		if msg.err != nil {
			m.err = fmt.Sprintf("session %s: %v", msg.name, msg.err)
		} else {
			m.err = ""
		}
		m.state = viewProjects
		return m, nil

	case sessionTickMsg:
		m.sessionMgr.RefreshStatus()
		return m, sessionTick()

	case projectAddedMsg:
		config.AddProject(msg.name, msg.path)
		m.state = viewProjects
		m.projectList.SetItems(loadProjects())
		m.err = ""
		return m, nil

	case projectDeletedMsg:
		m.projectList.SetItems(loadProjects())
		return m, nil

	case profileAddedMsg:
		// Save the new profile
		cfg, err := config.LoadConfig()
		if err == nil {
			cfg.Profiles = append(cfg.Profiles, msg.cfg)
			if len(cfg.Profiles) == 1 {
				cfg.Default = msg.cfg.Name
			}
			config.SaveConfig(cfg)
		}
		m.state = viewProfiles
		items, _ := loadProfiles()
		m.profileList.SetItems(items)
		m.cfg, _ = config.LoadConfig()
		m.err = ""
		return m, nil

	case profileSwitchedMsg:
		items, _ := loadProfiles()
		m.profileList.SetItems(items)
		m.cfg, _ = config.LoadConfig()
		return m, nil

	case settingChangedMsg:
		m.cfg, _ = config.LoadConfig()
		return m, nil
	}

	return m.updateList(msg)
}

func (m Model) updateAddForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.state = viewProjects
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.addForm, cmd = m.addForm.Update(msg)
	return m, cmd
}

func (m Model) updateProfileForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.state = viewProfiles
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.profileForm, cmd = m.profileForm.Update(msg)
	return m, cmd
}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.state = viewProjects
			m.focus = focusLeft
			return m, nil
		case "up", "k":
			if m.settings.cursor > 0 {
				m.settings.cursor--
			}
			return m, nil
		case "down", "j":
			if m.settings.cursor < len(m.settings.items)-1 {
				m.settings.cursor++
			}
			return m, nil
		case "enter", " ":
			item := &m.settings.items[m.settings.cursor]
			if item.options != nil {
				// Cycle to next option
				for i, opt := range item.options {
					if opt == item.value {
						item.value = item.options[(i+1)%len(item.options)]
						return m, m.applySetting(item.key, item.value)
					}
				}
				// If current value not found in options, set to first
				item.value = item.options[0]
				return m, m.applySetting(item.key, item.value)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) applySetting(key, value string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadConfig()
		if err != nil {
			return nil
		}
		switch key {
		case "terminal":
			cfg.Terminal = value
		case "defaultBehavior":
			cfg.DefaultBehavior = value
		case "skipPermissions":
			cfg.SkipPermissions = value == "on"
		}
		config.SaveConfig(cfg)
		return settingChangedMsg{}
	}
}

func (m Model) updateRightPanel(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h", "esc":
			m.focus = focusLeft
			return m, nil

		case "up":
			if m.sessionCursor > 0 {
				m.sessionCursor--
			}
			return m, nil

		case "down":
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				running := m.sessionMgr.GetRunningByProject(item.info.Name)
				// sessions + "New Session" option
				maxIdx := len(running)
				if m.sessionCursor < maxIdx {
					m.sessionCursor++
				}
			}
			return m, nil

		case "k":
			// Kill the selected session
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				running := m.sessionMgr.GetRunningByProject(item.info.Name)
				if m.sessionCursor < len(running) {
					m.sessionMgr.KillSession(running[m.sessionCursor].ID)
					// Adjust cursor after removal
					remaining := m.sessionMgr.GetRunningByProject(item.info.Name)
					if len(remaining) == 0 {
						m.focus = focusLeft
					} else if m.sessionCursor >= len(remaining) {
						m.sessionCursor = len(remaining) - 1
					}
				}
			}
			return m, nil

		case "enter":
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				running := m.sessionMgr.GetRunningByProject(item.info.Name)
				if m.sessionCursor < len(running) {
					// Focus existing session terminal
					m.sessionMgr.FocusSession()
					m.focus = focusLeft
					return m, nil
				}
				// "New Session" selected
				name := item.info.Name
				path := item.info.Path
				m.focus = focusLeft
				args, env := config.ClaudeCmdSpec()
				return m, func() tea.Msg {
					_, err := m.sessionMgr.StartSession(name, path, args, env)
					return sessionStartedMsg{name: name, err: err}
				}
			}

		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.state == viewProjects {
		m.projectList, cmd = m.projectList.Update(msg)
	} else if m.state == viewProfiles {
		m.profileList, cmd = m.profileList.Update(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Inner dimensions after appStyle padding (Padding(1,2) = 4 horizontal, 2 vertical)
	innerWidth := m.width - 4

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	if m.state == viewAddForm {
		b.WriteString(m.addForm.View())
	} else if m.state == viewAddProfile {
		b.WriteString(m.profileForm.View())
	} else if m.state == viewSettings {
		// Settings uses full width, no left/right split
		contentHeight := m.height - 10
		b.WriteString(m.settings.View(innerWidth, contentHeight))
	} else {
		// Main content: left list + right detail
		leftWidth := innerWidth / 2
		rightWidth := innerWidth - leftWidth - 2
		contentHeight := m.height - 10 // header + footer + appStyle padding

		var leftPanel, rightPanel string

		if m.state == viewProjects {
			leftPanel = m.projectList.View()
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				rightPanel = renderProjectDetail(item.info, rightWidth, contentHeight, m.sessionMgr, m.focus == focusRight, m.sessionCursor)
			}
		} else {
			leftPanel = m.profileList.View()
			if item, ok := m.profileList.SelectedItem().(profileItem); ok {
				rightPanel = renderProfileDetail(item, rightWidth, contentHeight)
			}
		}

		content := lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Width(leftWidth).Render(leftPanel),
			lipgloss.NewStyle().Width(rightWidth).MarginLeft(2).Render(rightPanel),
		)
		b.WriteString(content)
	}

	// Error display
	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(statusErrorStyle.Render("  Error: " + m.err))
	}

	// Footer help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(m.renderHelp()))

	return appStyle.Render(b.String())
}

func (m Model) renderHeader() string {
	innerWidth := m.width - 4

	title := titleStyle.Render(" ⬡ codes ")

	projectTab := inactiveTabStyle.Render("Projects")
	profileTab := inactiveTabStyle.Render("Profiles")
	settingsTab := inactiveTabStyle.Render("Settings")

	if m.state == viewProjects || m.state == viewAddForm {
		projectTab = activeTabStyle.Render("Projects")
	} else if m.state == viewProfiles || m.state == viewAddProfile {
		profileTab = activeTabStyle.Render("Profiles")
	} else if m.state == viewSettings {
		settingsTab = activeTabStyle.Render("Settings")
	}

	defaultCfg := ""
	if m.cfg != nil && m.cfg.Default != "" {
		term := config.GetTerminal()
		if term == "" {
			term = "terminal"
		}
		defaultCfg = lipgloss.NewStyle().
			Foreground(mutedColor).
			Render(fmt.Sprintf("Profile: %s | Term: %s", m.cfg.Default, term))
	}

	// Show running session count
	running := m.sessionMgr.RunningCount()
	sessionInfo := ""
	if running > 0 {
		sessionInfo = statusOkStyle.Render(fmt.Sprintf(" [%d running]", running))
	}

	tabs := fmt.Sprintf("%s  %s  %s", projectTab, profileTab, settingsTab)
	gap := strings.Repeat(" ", max(0, innerWidth-lipgloss.Width(title)-lipgloss.Width(tabs)-lipgloss.Width(defaultCfg)-lipgloss.Width(sessionInfo)-8))

	return fmt.Sprintf("%s  %s%s%s%s", title, tabs, gap, sessionInfo, defaultCfg)
}

func (m Model) renderHelp() string {
	if m.state == viewAddForm {
		return formHintStyle.Render("Tab: switch fields  Enter: add  Esc: cancel")
	}
	if m.state == viewAddProfile {
		return formHintStyle.Render("Tab: switch fields  Space: toggle  Enter: add  Esc: cancel")
	}
	if m.state == viewSettings {
		return formHintStyle.Render("↑↓ select  Enter/Space cycle  tab switch  q quit")
	}
	if m.focus == focusRight && m.state == viewProjects {
		return formHintStyle.Render("↑↓ select  Enter open  k kill  ← back  q quit")
	}

	parts := []string{
		"↑↓ select",
		"enter open",
		"/ filter",
	}

	if m.state == viewProjects {
		parts = append(parts, "→ sessions", "a add", "d delete", "k kill", "t terminal")
	}
	if m.state == viewProfiles {
		parts = append(parts, "a add profile")
	}

	parts = append(parts, "tab switch", "q quit")

	return strings.Join(parts, "  ")
}

// Run starts the TUI application.
func Run() error {
	m := NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
