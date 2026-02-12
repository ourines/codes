package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codes/internal/config"
	"codes/internal/remote"
	"codes/internal/session"
)

type viewState int

const (
	viewProjects   viewState = iota
	viewProfiles
	viewRemotes
	viewSettings
	viewAddForm
	viewAddProfile
	viewAddRemote
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
	remoteList    list.Model
	addForm       addFormModel
	profileForm   profileFormModel
	remoteForm    remoteFormModel
	help          help.Model
	cfg           *config.Config
	width         int
	height        int
	err           string
	statusMsg     string // non-error status/loading message
	sessionMgr    *session.Manager
	sessionCursor int // cursor index within right-panel session list
	settings      settingsModel
	remoteStatus  map[string]*remote.RemoteStatus
}

// projectDeletedMsg is sent after deleting a project.
type projectDeletedMsg struct{ name string }

// editorOpenedMsg is sent after attempting to open a project in an editor.
type editorOpenedMsg struct {
	name string
	err  error
}

// browserOpenedMsg is sent after attempting to open a URL in the browser.
type browserOpenedMsg struct {
	url string
	err error
}

// profileSwitchedMsg is sent after switching the default profile.
type profileSwitchedMsg struct{ name string }

// sessionStartedMsg is sent after attempting to start a session.
type sessionStartedMsg struct {
	name string
	err  error
}

// sessionTickMsg triggers periodic session status refresh.
type sessionTickMsg struct{}

// remoteDeletedMsg is sent after deleting a remote.
type remoteDeletedMsg struct{ name string }

// remoteStatusMsg is sent after checking remote status.
type remoteStatusMsg struct {
	name   string
	status *remote.RemoteStatus
	err    error
}

// remoteSyncMsg is sent after syncing profiles to a remote.
type remoteSyncMsg struct {
	name   string
	status *remote.RemoteStatus
	err    error
}

// remoteSetupMsg is sent after running full setup on a remote.
type remoteSetupMsg struct {
	name   string
	status *remote.RemoteStatus
	err    error
}

// remoteStatusTickMsg triggers periodic remote status refresh.
type remoteStatusTickMsg struct{}

// remoteStatusRefreshDoneMsg carries refreshed statuses from background check.
type remoteStatusRefreshDoneMsg struct {
	statuses map[string]*remote.RemoteStatus
}

func sessionTick() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return sessionTickMsg{}
	})
}

func remoteStatusTick() tea.Cmd {
	return tea.Tick(60*time.Second, func(t time.Time) tea.Msg {
		return remoteStatusTickMsg{}
	})
}

// NewModel creates the initial TUI model.
func NewModel() Model {
	// Load projects
	projectItems := loadProjects()
	projectDelegate := newStyledDelegate()
	projectDelegate.ShowDescription = true
	pl := list.New(projectItems, projectDelegate, 0, 0)
	pl.SetShowTitle(false)
	pl.SetShowHelp(false)
	pl.SetShowStatusBar(false)
	pl.SetFilteringEnabled(true)

	// Load profiles
	profileItems, _ := loadProfiles()
	profileDelegate := newStyledDelegate()
	profileDelegate.ShowDescription = true
	cl := list.New(profileItems, profileDelegate, 0, 0)
	cl.SetShowTitle(false)
	cl.SetShowHelp(false)
	cl.SetShowStatusBar(false)
	cl.SetFilteringEnabled(true)

	// Load remotes
	remoteItems := loadRemotes()
	remoteDelegate := newStyledDelegate()
	remoteDelegate.ShowDescription = true
	rl := list.New(remoteItems, remoteDelegate, 0, 0)
	rl.SetShowTitle(false)
	rl.SetShowHelp(false)
	rl.SetShowStatusBar(false)
	rl.SetFilteringEnabled(true)

	cfg, _ := config.LoadConfig()

	return Model{
		state:        viewProjects,
		projectList:  pl,
		profileList:  cl,
		remoteList:   rl,
		help:         help.New(),
		cfg:          cfg,
		sessionMgr:   session.NewManager(config.GetTerminal()),
		settings:     newSettingsModel(cfg),
		remoteStatus: remote.LoadStatusCache(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(sessionTick(), remoteStatusTick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Account for appStyle padding: Padding(1, 2) = 2 vertical, 4 horizontal
		innerWidth := m.width - 4
		innerHeight := m.height - 2

		// Overhead within inner area: header(1) + gap(1) + help(2) + status(1) = 5
		listHeight := innerHeight - 5
		listWidth := innerWidth / 2
		m.projectList.SetSize(listWidth, listHeight)
		m.profileList.SetSize(listWidth, listHeight)
		m.remoteList.SetSize(listWidth, listHeight)
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
		if m.state == viewAddRemote {
			return m.updateRemoteForm(msg)
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
		if m.state == viewRemotes && m.remoteList.FilterState() == list.Filtering {
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
				m.state = viewRemotes
			case viewRemotes:
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

		case msg.String() == "a" && m.state == viewRemotes:
			m.state = viewAddRemote
			m.remoteForm = newRemoteForm()
			return m, nil

		case msg.String() == "d" && m.state == viewRemotes:
			if item, ok := m.remoteList.SelectedItem().(remoteItem); ok {
				name := item.host.Name
				return m, func() tea.Msg {
					config.RemoveRemote(name)
					return remoteDeletedMsg{name: name}
				}
			}

		case msg.String() == "t" && m.state == viewRemotes:
			// Test connection
			if item, ok := m.remoteList.SelectedItem().(remoteItem); ok {
				name := item.host.Name
				host := item.host
				m.statusMsg = fmt.Sprintf("testing %s...", name)
				return m, func() tea.Msg {
					status, err := remote.CheckRemoteStatus(&host)
					return remoteStatusMsg{name: name, status: status, err: err}
				}
			}

		case msg.String() == "s" && m.state == viewRemotes:
			// Sync profiles
			if item, ok := m.remoteList.SelectedItem().(remoteItem); ok {
				name := item.host.Name
				host := item.host
				m.statusMsg = fmt.Sprintf("syncing %s...", name)
				return m, func() tea.Msg {
					if err := remote.SyncProfiles(&host); err != nil {
						return remoteSyncMsg{name: name, err: err}
					}
					// Auto-refresh status after sync
					status, _ := remote.CheckRemoteStatus(&host)
					return remoteSyncMsg{name: name, status: status}
				}
			}

		case msg.String() == "S" && m.state == viewRemotes:
			// Full setup (install + sync)
			if item, ok := m.remoteList.SelectedItem().(remoteItem); ok {
				name := item.host.Name
				host := item.host
				m.statusMsg = fmt.Sprintf("setting up %s...", name)
				return m, func() tea.Msg {
					// Step 1: Install codes
					if _, err := remote.InstallOnRemote(&host); err != nil {
						return remoteSetupMsg{name: name, err: err}
					}
					// Step 2: Install Claude CLI (non-fatal)
					remote.InstallClaudeOnRemote(&host)
					// Step 3: Sync profiles
					if err := remote.SyncProfiles(&host); err != nil {
						return remoteSetupMsg{name: name, err: err}
					}
					// Auto-refresh status after setup
					status, _ := remote.CheckRemoteStatus(&host)
					return remoteSetupMsg{name: name, status: status}
				}
			}

		case msg.String() == "d" && m.state == viewProjects:
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				return m, func() tea.Msg {
					config.RemoveProject(item.info.Name)
					return projectDeletedMsg{name: item.info.Name}
				}
			}

		case msg.String() == "x" && m.state == viewProjects:
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				m.sessionMgr.KillByProject(item.info.Name)
			}
			return m, nil

		case msg.String() == "e" && m.state == viewProjects:
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				if !item.info.Exists {
					return m, nil
				}
				if item.info.Remote != "" {
					m.err = "cannot open remote project in local editor"
					return m, nil
				}
				path := item.info.Path
				name := item.info.Name
				return m, func() tea.Msg {
					editor := detectEditor()
					if editor == "" {
						return editorOpenedMsg{err: fmt.Errorf("no editor found; set one with: codes config set editor <cmd>")}
					}
					cmd := exec.Command(editor, path)
					err := cmd.Start()
					return editorOpenedMsg{name: name, err: err}
				}
			}

		case msg.String() == "g" && m.state == viewProjects:
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				if !item.info.Exists {
					return m, nil
				}
				path := item.info.Path
				remoteName := item.info.Remote
				return m, func() tea.Msg {
					var gitURL string
					if remoteName != "" {
						host, ok := config.GetRemote(remoteName)
						if !ok {
							return browserOpenedMsg{err: fmt.Errorf("remote '%s' not found", remoteName)}
						}
						out, err := remote.RunSSH(host, fmt.Sprintf("git -C %q remote get-url origin", path))
						if err != nil {
							return browserOpenedMsg{err: fmt.Errorf("no remote origin on %s", remoteName)}
						}
						gitURL = strings.TrimSpace(out)
					} else {
						cmd := exec.Command("git", "-C", path, "remote", "get-url", "origin")
						out, err := cmd.Output()
						if err != nil {
							return browserOpenedMsg{err: fmt.Errorf("not a git repo or no remote origin")}
						}
						gitURL = strings.TrimSpace(string(out))
					}
					browserURL := gitURLToBrowserURL(gitURL)
					if browserURL == "" {
						return browserOpenedMsg{err: fmt.Errorf("could not parse remote URL: %s", gitURL)}
					}
					err := openBrowser(browserURL)
					return browserOpenedMsg{url: browserURL, err: err}
				}
			}

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

					// Remote project → SSH session in new terminal
					if item.info.Remote != "" {
						host, ok := config.GetRemote(item.info.Remote)
						if !ok {
							m.err = fmt.Sprintf("remote '%s' not found", item.info.Remote)
							return m, nil
						}
						return m, func() tea.Msg {
							_, err := m.sessionMgr.StartRemoteSession(name, host, path)
							return sessionStartedMsg{name: name, err: err}
						}
					}

					// Local project → session in new terminal
					args, env := config.ClaudeCmdSpec()
					return m, func() tea.Msg {
						_, err := m.sessionMgr.StartSession(name, path, args, env)
						return sessionStartedMsg{name: name, err: err}
					}
				}
			} else if m.state == viewProfiles {
				if item, ok := m.profileList.SelectedItem().(profileItem); ok {
					profileName := item.cfg.Name
					return m, func() tea.Msg {
						cfg, err := config.LoadConfig()
						if err == nil {
							cfg.Default = profileName
							config.SaveConfig(cfg)
						}
						return profileSwitchedMsg{name: profileName}
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

	case editorOpenedMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.statusMsg = fmt.Sprintf("opened %s in editor", msg.name)
		}
		return m, nil

	case browserOpenedMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.statusMsg = fmt.Sprintf("opened %s", msg.url)
		}
		return m, nil

	case sessionTickMsg:
		m.sessionMgr.RefreshStatus()
		return m, sessionTick()

	case projectAddedMsg:
		config.AddProjectEntry(msg.name, config.ProjectEntry{Path: msg.path, Remote: msg.remote})
		m.state = viewProjects
		m.projectList.SetItems(loadProjects())
		m.err = ""
		return m, nil

	case pathDebounceMsg:
		// Only process if still in add form, sequence matches, and remote is selected
		if m.state != viewAddForm || msg.seq != m.addForm.debounceSeq || m.addForm.remoteIdx < 0 {
			return m, nil
		}
		remoteName := m.addForm.selectedRemote()
		host, ok := config.GetRemote(remoteName)
		if !ok {
			return m, nil
		}
		m.addForm.loadingSugg = true
		path := msg.path
		return m, func() tea.Msg {
			// Determine dir and prefix from path
			var dir, prefix string
			if strings.HasSuffix(path, "/") {
				dir = path
				prefix = ""
			} else {
				lastSlash := strings.LastIndex(path, "/")
				if lastSlash >= 0 {
					dir = path[:lastSlash+1]
					prefix = path[lastSlash+1:]
				} else {
					dir = "/"
					prefix = path
				}
			}

			entries, err := remote.ListRemoteDir(host, dir)
			if err != nil {
				return pathSuggestionsMsg{forPath: path}
			}
			suggestions := listRemotePathSuggestions(strings.TrimSuffix(dir, "/"), entries, prefix)
			return pathSuggestionsMsg{suggestions: suggestions, forPath: path}
		}

	case pathSuggestionsMsg:
		if m.state == viewAddForm && msg.forPath == m.addForm.pathInput.Value() {
			m.addForm.suggestions = msg.suggestions
			m.addForm.suggIdx = 0
			m.addForm.loadingSugg = false
		}
		return m, nil

	case gitCloneStartMsg:
		m.statusMsg = "Cloning..."
		m.state = viewProjects
		gitURL := msg.gitURL
		clonePath := msg.clonePath
		name := msg.name
		remoteName := msg.remote

		return m, func() tea.Msg {
			if remoteName != "" {
				return cloneRemote(remoteName, gitURL, clonePath, name)
			}
			return cloneLocal(gitURL, clonePath, name)
		}

	case gitCloneMsg:
		m.statusMsg = ""
		if msg.err != nil {
			m.err = fmt.Sprintf("clone failed: %v", msg.err)
			return m, nil
		}
		// Auto-add the cloned repo as a project
		config.AddProjectEntry(msg.name, config.ProjectEntry{Path: msg.path, Remote: msg.remote})
		m.projectList.SetItems(loadProjects())
		m.err = ""
		m.statusMsg = fmt.Sprintf("✓ cloned and added %s", msg.name)
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

	case remoteAddedMsg:
		config.AddRemote(msg.host)
		m.state = viewRemotes
		m.remoteList.SetItems(loadRemotes())
		m.err = ""
		return m, nil

	case remoteDeletedMsg:
		m.remoteList.SetItems(loadRemotes())
		delete(m.remoteStatus, msg.name)
		remote.DeleteStatusCache(msg.name)
		return m, nil

	case remoteStatusMsg:
		m.statusMsg = ""
		if msg.err != nil {
			m.err = fmt.Sprintf("remote %s: %v", msg.name, msg.err)
		} else {
			m.remoteStatus[msg.name] = msg.status
			remote.UpdateStatusCache(msg.name, msg.status)
			m.err = ""
			m.statusMsg = fmt.Sprintf("✓ %s connected", msg.name)
		}
		return m, nil

	case remoteSyncMsg:
		m.statusMsg = ""
		if msg.err != nil {
			m.err = fmt.Sprintf("sync %s: %v", msg.name, msg.err)
		} else {
			m.err = ""
			if msg.status != nil {
				m.remoteStatus[msg.name] = msg.status
				remote.UpdateStatusCache(msg.name, msg.status)
			}
			m.statusMsg = fmt.Sprintf("✓ %s synced", msg.name)
		}
		return m, nil

	case remoteSetupMsg:
		m.statusMsg = ""
		if msg.err != nil {
			m.err = fmt.Sprintf("setup %s: %v", msg.name, msg.err)
		} else {
			m.err = ""
			if msg.status != nil {
				m.remoteStatus[msg.name] = msg.status
				remote.UpdateStatusCache(msg.name, msg.status)
			}
			m.statusMsg = fmt.Sprintf("✓ %s setup complete", msg.name)
		}
		return m, nil

	case remoteStatusTickMsg:
		// Auto-refresh: check status for all configured remotes in background
		remotes, _ := config.ListRemotes()
		if len(remotes) == 0 {
			return m, remoteStatusTick()
		}
		return m, tea.Batch(
			func() tea.Msg {
				results := make(map[string]*remote.RemoteStatus)
				for _, r := range remotes {
					host := r
					status, err := remote.CheckRemoteStatus(&host)
					if err == nil && status != nil {
						results[host.Name] = status
						remote.UpdateStatusCache(host.Name, status)
					}
				}
				return remoteStatusRefreshDoneMsg{statuses: results}
			},
			remoteStatusTick(),
		)

	case remoteStatusRefreshDoneMsg:
		for name, status := range msg.statuses {
			m.remoteStatus[name] = status
		}
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

func (m Model) updateRemoteForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.state = viewRemotes
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.remoteForm, cmd = m.remoteForm.Update(msg)
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
		case "projects_dir":
			cfg.ProjectsDir = value
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

		case "x":
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
	} else if m.state == viewRemotes {
		m.remoteList, cmd = m.remoteList.Update(msg)
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
	b.WriteString("\n")

	if m.state == viewAddForm {
		b.WriteString(m.addForm.View())
	} else if m.state == viewAddProfile {
		b.WriteString(m.profileForm.View())
	} else if m.state == viewAddRemote {
		b.WriteString(m.remoteForm.View())
	} else if m.state == viewSettings {
		// Settings uses full width, no left/right split
		contentHeight := m.height - 7
		b.WriteString(m.settings.View(innerWidth, contentHeight))
	} else {
		// Main content: left list + right detail
		leftWidth := innerWidth / 2
		rightWidth := innerWidth - leftWidth - 2
		contentHeight := m.height - 7 // appStyle(2) + header(1) + gap(1) + help(2) + status(1)

		var leftPanel, rightPanel string

		if m.state == viewProjects {
			leftPanel = m.projectList.View()
			if item, ok := m.projectList.SelectedItem().(projectItem); ok {
				rightPanel = renderProjectDetail(item.info, rightWidth, contentHeight, m.sessionMgr, m.focus == focusRight, m.sessionCursor)
			}
		} else if m.state == viewRemotes {
			leftPanel = m.remoteList.View()
			if item, ok := m.remoteList.SelectedItem().(remoteItem); ok {
				status := m.remoteStatus[item.host.Name]
				rightPanel = renderRemoteDetail(item.host, rightWidth, contentHeight, status)
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

	// Status/Error display
	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(statusErrorStyle.Render("  Error: " + m.err))
	} else if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(statusOkStyle.Render("  " + m.statusMsg))
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
	remotesTab := inactiveTabStyle.Render("Remotes")
	settingsTab := inactiveTabStyle.Render("Settings")

	if m.state == viewProjects || m.state == viewAddForm {
		projectTab = activeTabStyle.Render("Projects")
	} else if m.state == viewProfiles || m.state == viewAddProfile {
		profileTab = activeTabStyle.Render("Profiles")
	} else if m.state == viewRemotes || m.state == viewAddRemote {
		remotesTab = activeTabStyle.Render("Remotes")
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

	tabs := fmt.Sprintf("%s  %s  %s  %s", projectTab, profileTab, remotesTab, settingsTab)
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
	if m.state == viewAddRemote {
		return formHintStyle.Render("Tab: switch fields  Enter: add  Esc: cancel")
	}
	if m.state == viewSettings {
		return formHintStyle.Render("↑↓ select  Enter/Space cycle  tab switch  q quit")
	}
	if m.focus == focusRight && m.state == viewProjects {
		return formHintStyle.Render("↑↓/jk select  Enter open  x kill  ← back  q quit")
	}

	parts := []string{
		"jk/↑↓ select",
		"enter open",
		"/ filter",
	}

	if m.state == viewProjects {
		parts = append(parts, "→/l sessions", "a add", "d delete", "x kill", "e editor", "g github", "t terminal")
	}
	if m.state == viewProfiles {
		parts = append(parts, "a add profile")
	}
	if m.state == viewRemotes {
		parts = append(parts, "a add", "d delete", "t test", "s sync", "S setup")
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

// cloneLocal tries to clone a git repo locally with smart fallback:
// 1. Try gh repo clone (if gh CLI available)
// 2. Try git clone with original URL
// 3. If HTTPS URL failed, retry with SSH URL
// 4. Return guidance on failure
func cloneLocal(gitURL, clonePath, name string) tea.Msg {
	hasGH := false
	if _, err := exec.LookPath("gh"); err == nil {
		hasGH = true
	}

	// Step 1: Try gh repo clone
	if hasGH {
		cmd := exec.Command("gh", "repo", "clone", gitURL, clonePath)
		if out, err := cmd.CombinedOutput(); err == nil {
			return gitCloneMsg{name: name, path: clonePath}
		} else {
			_ = out // gh failed, continue to git clone
		}
	}

	// Step 2: Try git clone with original URL
	cmd := exec.Command("git", "clone", gitURL, clonePath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return gitCloneMsg{name: name, path: clonePath}
	}

	// Step 3: If HTTPS URL failed, try SSH URL
	if isHTTPSURL(gitURL) {
		sshURL := httpsToSSH(gitURL)
		cmd2 := exec.Command("git", "clone", sshURL, clonePath)
		if out2, err2 := cmd2.CombinedOutput(); err2 == nil {
			return gitCloneMsg{name: name, path: clonePath}
		} else {
			_ = out2
		}
	}

	// Step 4: All failed — return guidance
	detail := strings.TrimSpace(string(out))
	var guidance string
	if hasGH {
		guidance = "Clone failed. To clone private repositories:\n" +
			"  • GitHub CLI: gh auth login\n" +
			"  • SSH key:   ssh-keygen -t ed25519 && ssh-add\n" +
			"  • HTTPS:     git config --global credential.helper store"
	} else {
		guidance = "Clone failed. To clone private repositories:\n" +
			"  • Install GitHub CLI (gh) for easy auth: https://cli.github.com\n" +
			"  • SSH key:   ssh-keygen -t ed25519 && ssh-add\n" +
			"  • HTTPS:     git config --global credential.helper store"
	}
	if detail != "" {
		return gitCloneMsg{err: fmt.Errorf("%s\n\n%s", detail, guidance)}
	}
	return gitCloneMsg{err: fmt.Errorf("%s", guidance)}
}

// cloneRemote tries to clone a git repo on a remote host with smart fallback:
// 1. Try git clone with SSH agent forwarding (-A)
// 2. If HTTPS URL failed, retry with SSH URL (still with -A)
// 3. Return guidance on failure
func cloneRemote(remoteName, gitURL, clonePath, name string) tea.Msg {
	host, ok := config.GetRemote(remoteName)
	if !ok {
		return gitCloneMsg{err: fmt.Errorf("remote '%s' not found", remoteName)}
	}

	// Step 1: Try git clone with agent forwarding
	_, err := remote.RunSSHWithAgent(host, fmt.Sprintf("git clone %q %q", gitURL, clonePath))
	if err == nil {
		return gitCloneMsg{name: name, path: clonePath, remote: remoteName}
	}
	firstErr := err

	// Step 2: If HTTPS URL failed, try SSH URL
	if isHTTPSURL(gitURL) {
		sshURL := httpsToSSH(gitURL)
		_, err2 := remote.RunSSHWithAgent(host, fmt.Sprintf("git clone %q %q", sshURL, clonePath))
		if err2 == nil {
			return gitCloneMsg{name: name, path: clonePath, remote: remoteName}
		}
	}

	// Step 3: All failed — return guidance
	guidance := fmt.Sprintf("Clone failed on remote. To clone private repositories on %s:\n"+
		"  • Forward SSH agent: ensure ssh-agent is running locally (ssh-add -l)\n"+
		"  • Or set up SSH key on remote: codes remote ssh %s, then ssh-keygen",
		host.UserAtHost(), remoteName)
	return gitCloneMsg{err: fmt.Errorf("%v\n\n%s", firstErr, guidance)}
}

// detectEditor returns the editor command to use, checking:
// 1. Config setting  2. $VISUAL  3. $EDITOR  4. Auto-detect from PATH
func detectEditor() string {
	if e := config.GetEditor(); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	for _, candidate := range []string{"cursor", "code", "zed", "subl", "nvim", "vim"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// gitURLToBrowserURL converts a git remote URL to a browser-friendly HTTPS URL.
func gitURLToBrowserURL(gitURL string) string {
	gitURL = strings.TrimSpace(gitURL)
	// git@github.com:user/repo.git → https://github.com/user/repo
	if strings.HasPrefix(gitURL, "git@") {
		gitURL = strings.TrimPrefix(gitURL, "git@")
		gitURL = strings.Replace(gitURL, ":", "/", 1)
		gitURL = strings.TrimSuffix(gitURL, ".git")
		return "https://" + gitURL
	}
	// ssh://git@github.com/user/repo.git → https://github.com/user/repo
	if strings.HasPrefix(gitURL, "ssh://") {
		gitURL = strings.TrimPrefix(gitURL, "ssh://")
		gitURL = strings.TrimPrefix(gitURL, "git@")
		gitURL = strings.TrimSuffix(gitURL, ".git")
		return "https://" + gitURL
	}
	// https://github.com/user/repo.git → https://github.com/user/repo
	if strings.HasPrefix(gitURL, "https://") || strings.HasPrefix(gitURL, "http://") {
		return strings.TrimSuffix(gitURL, ".git")
	}
	return ""
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}
