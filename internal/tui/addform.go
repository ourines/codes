package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"codes/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// projectAddedMsg is sent when the user submits the add form.
type projectAddedMsg struct {
	name   string
	path   string
	remote string // remote host name, empty = local
}

// pathSuggestion represents a single path completion suggestion.
type pathSuggestion struct {
	display string // basename (with "/" suffix for dirs)
	full    string // full absolute path
	isDir   bool
}

// pathSuggestionsMsg carries async path suggestions (from SSH).
type pathSuggestionsMsg struct {
	suggestions []pathSuggestion
	forPath     string // used to check if still relevant
}

// pathDebounceMsg triggers SSH path completion after debounce delay.
type pathDebounceMsg struct {
	path string
	seq  int
}

// gitCloneStartMsg signals the start of a git clone operation.
type gitCloneStartMsg struct {
	gitURL    string
	clonePath string
	name      string
	remote    string
}

// gitCloneMsg carries the result of a git clone operation.
type gitCloneMsg struct {
	name   string
	path   string
	remote string
	err    error
}

// addFormModel is the model for the add-project form.
type addFormModel struct {
	nameInput      textinput.Model
	pathInput      textinput.Model
	clonePathInput textinput.Model // target path for git clone
	remoteNames    []string        // available remote host names
	remoteIdx      int             // -1 = local, 0..n = index into remoteNames
	focused        int             // field index
	err            string

	// Path completion
	suggestions []pathSuggestion
	suggIdx     int
	lastPath    string // last scanned path value
	loadingSugg bool   // SSH suggestions loading

	// Debounce for SSH path completion
	debounceSeq int

	// Git mode
	isGitMode bool
	gitURL    string
}

// newAddForm creates a new add-project form.
func newAddForm() addFormModel {
	ni := textinput.New()
	ni.Placeholder = "optional, defaults to folder name"
	ni.CharLimit = 50
	ni.Focus()

	pi := textinput.New()
	pi.Placeholder = "/path/to/project or git URL"
	pi.CharLimit = 300

	ci := textinput.New()
	ci.Placeholder = "clone target path"
	ci.CharLimit = 200

	// Load available remotes
	var remoteNames []string
	if remotes, err := config.ListRemotes(); err == nil {
		for _, r := range remotes {
			remoteNames = append(remoteNames, r.Name)
		}
	}

	return addFormModel{
		nameInput:      ni,
		pathInput:      pi,
		clonePathInput: ci,
		remoteNames:    remoteNames,
		remoteIdx:      -1, // default: local
		focused:        0,
	}
}

func (m *addFormModel) fieldCount() int {
	base := 2 // name, path
	if m.isGitMode {
		base = 3 // name, path/URL, clone path
	}
	if len(m.remoteNames) > 0 {
		base++ // remote selector
	}
	return base
}

func (m *addFormModel) remoteFieldIdx() int {
	if m.isGitMode {
		return 3
	}
	return 2
}

// focusInput updates Focus/Blur state on the inputs based on m.focused.
func (m *addFormModel) focusInput() {
	m.nameInput.Blur()
	m.pathInput.Blur()
	m.clonePathInput.Blur()

	switch m.focused {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.pathInput.Focus()
	case 2:
		if m.isGitMode {
			m.clonePathInput.Focus()
		}
		// else: remote field (no text input)
	}
}

func (m *addFormModel) selectedRemote() string {
	if m.remoteIdx < 0 || m.remoteIdx >= len(m.remoteNames) {
		return ""
	}
	return m.remoteNames[m.remoteIdx]
}

// Git URL detection patterns.
var gitURLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^git@[^:]+:.+/.+`),
	regexp.MustCompile(`^https?://[^/]+/.+/.+`),
	regexp.MustCompile(`^ssh://git@.+/.+`),
}

func isGitURL(input string) bool {
	input = strings.TrimSpace(input)
	for _, pat := range gitURLPatterns {
		if pat.MatchString(input) {
			return true
		}
	}
	return false
}

func repoNameFromURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(url, ".git")

	// git@host:user/repo
	if strings.HasPrefix(url, "git@") {
		if idx := strings.LastIndex(url, "/"); idx >= 0 {
			return url[idx+1:]
		}
		if idx := strings.LastIndex(url, ":"); idx >= 0 {
			return url[idx+1:]
		}
	}

	// https://host/user/repo or ssh://git@host/user/repo
	if idx := strings.LastIndex(url, "/"); idx >= 0 {
		return url[idx+1:]
	}
	return url
}

// checkGitMode updates the git mode state based on the current path input.
func (m *addFormModel) checkGitMode() {
	val := strings.TrimSpace(m.pathInput.Value())
	wasGitMode := m.isGitMode

	if isGitURL(val) {
		m.isGitMode = true
		m.gitURL = val
		if !wasGitMode {
			// Auto-fill clone path on entering git mode
			repoName := repoNameFromURL(val)
			projectsDir := config.GetProjectsDir()
			m.clonePathInput.SetValue(filepath.Join(projectsDir, repoName))
			m.clonePathInput.CursorEnd()
		}
		// Clear path suggestions in git mode
		m.suggestions = nil
	} else {
		m.isGitMode = false
		m.gitURL = ""
	}
}

// listPathSuggestions scans the local filesystem and returns matching path suggestions.
func listPathSuggestions(input string) []pathSuggestion {
	if input == "" {
		return nil
	}

	expanded := input
	// Expand ~ to home directory
	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		expanded = home + expanded[1:]
	}

	var dir, prefix string
	if strings.HasSuffix(expanded, "/") {
		dir = expanded
		prefix = ""
	} else {
		dir = filepath.Dir(expanded)
		prefix = filepath.Base(expanded)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// Show hidden files only if user explicitly typed a dot prefix
	showHidden := strings.HasPrefix(prefix, ".")

	var suggestions []pathSuggestion
	for _, e := range entries {
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}
		full := filepath.Join(dir, name)
		display := name
		isDir := e.IsDir()
		if isDir {
			display += "/"
		}
		suggestions = append(suggestions, pathSuggestion{
			display: display,
			full:    full,
			isDir:   isDir,
		})
		if len(suggestions) >= 6 {
			break
		}
	}
	return suggestions
}

// listRemotePathSuggestions builds suggestions from SSH ls output entries.
func listRemotePathSuggestions(dir string, entries []string, prefix string) []pathSuggestion {
	showHidden := strings.HasPrefix(prefix, ".")

	var suggestions []pathSuggestion
	for _, entry := range entries {
		isDir := strings.HasSuffix(entry, "/")
		name := strings.TrimSuffix(entry, "/")
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}
		full := dir + "/" + name
		display := entry // keep the / suffix for dirs
		suggestions = append(suggestions, pathSuggestion{
			display: display,
			full:    full,
			isDir:   isDir,
		})
		if len(suggestions) >= 6 {
			break
		}
	}
	return suggestions
}

// updateSuggestions refreshes the suggestion list based on current path input.
// For remote paths, returns a debounced tea.Cmd.
func (m *addFormModel) updateSuggestions() tea.Cmd {
	path := m.pathInput.Value()
	if path == m.lastPath {
		return nil
	}
	m.lastPath = path

	// Don't suggest paths in git mode
	if m.isGitMode {
		m.suggestions = nil
		return nil
	}

	if m.remoteIdx < 0 {
		// Local: synchronous
		m.suggestions = listPathSuggestions(path)
		m.suggIdx = 0
		return nil
	}

	// Remote: debounce 150ms
	m.debounceSeq++
	seq := m.debounceSeq
	m.suggestions = nil
	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		return pathDebounceMsg{path: path, seq: seq}
	})
}

// completeSuggestion applies the selected suggestion to the path input.
func (m *addFormModel) completeSuggestion() {
	if m.suggIdx < 0 || m.suggIdx >= len(m.suggestions) {
		return
	}
	sugg := m.suggestions[m.suggIdx]
	newPath := sugg.full
	if sugg.isDir {
		newPath += "/"
	}
	m.pathInput.SetValue(newPath)
	m.pathInput.CursorEnd()
	m.lastPath = newPath

	if m.remoteIdx < 0 {
		// Local: refresh suggestions synchronously
		m.suggestions = listPathSuggestions(newPath)
	} else {
		// Remote: clear and wait for debounce on next input
		m.suggestions = nil
	}
	m.suggIdx = 0
}

// Update handles input for the add form.
func (m addFormModel) Update(msg tea.Msg) (addFormModel, tea.Cmd) {
	fields := m.fieldCount()
	remoteField := m.remoteFieldIdx()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Path field with suggestions: intercept navigation keys
		if m.focused == 1 && len(m.suggestions) > 0 {
			switch key {
			case "tab":
				m.completeSuggestion()
				return m, nil
			case "down", "ctrl+n":
				m.suggIdx = (m.suggIdx + 1) % len(m.suggestions)
				return m, nil
			case "up", "ctrl+p":
				m.suggIdx = (m.suggIdx - 1 + len(m.suggestions)) % len(m.suggestions)
				return m, nil
			}
		}

		switch key {
		case "tab", "down":
			m.focused = (m.focused + 1) % fields
			m.focusInput()
			if m.focused != 1 {
				m.suggestions = nil
			}
			return m, nil
		case "shift+tab", "up":
			m.focused = (m.focused - 1 + fields) % fields
			m.focusInput()
			if m.focused != 1 {
				m.suggestions = nil
			}
			return m, nil
		case "left":
			if m.focused == remoteField && len(m.remoteNames) > 0 {
				m.remoteIdx--
				if m.remoteIdx < -1 {
					m.remoteIdx = len(m.remoteNames) - 1
				}
				// Reset path suggestions when remote changes
				m.lastPath = ""
				m.suggestions = nil
				return m, nil
			}
		case "right":
			if m.focused == remoteField && len(m.remoteNames) > 0 {
				m.remoteIdx++
				if m.remoteIdx >= len(m.remoteNames) {
					m.remoteIdx = -1
				}
				m.lastPath = ""
				m.suggestions = nil
				return m, nil
			}
		case "enter":
			// Remote field: cycle options
			if m.focused == remoteField && len(m.remoteNames) > 0 {
				m.remoteIdx++
				if m.remoteIdx >= len(m.remoteNames) {
					m.remoteIdx = -1
				}
				m.lastPath = ""
				m.suggestions = nil
				return m, nil
			}

			// Git clone mode: start clone
			if m.isGitMode {
				clonePath := strings.TrimSpace(m.clonePathInput.Value())
				if clonePath == "" {
					m.err = "Clone path is required"
					return m, nil
				}
				name := strings.TrimSpace(m.nameInput.Value())
				if name == "" {
					name = repoNameFromURL(m.gitURL)
				}
				// Expand ~ in clone path
				if strings.HasPrefix(clonePath, "~") {
					if home, err := os.UserHomeDir(); err == nil {
						clonePath = home + clonePath[1:]
					}
				}
				m.err = ""
				remoteName := m.selectedRemote()
				gitURL := m.gitURL
				return m, func() tea.Msg {
					return gitCloneStartMsg{
						gitURL:    gitURL,
						clonePath: clonePath,
						name:      name,
						remote:    remoteName,
					}
				}
			}

			// Normal add mode
			name := strings.TrimSpace(m.nameInput.Value())
			path := strings.TrimSpace(m.pathInput.Value())
			if path == "" {
				m.err = "Path is required"
				return m, nil
			}
			// Default name to the last directory component of the path
			if name == "" {
				name = filepath.Base(strings.TrimRight(path, "/"))
				if name == "" || name == "." || name == "/" {
					m.err = "Cannot derive name from path"
					return m, nil
				}
			}
			// Expand ~ before saving
			if strings.HasPrefix(path, "~") {
				if home, err := os.UserHomeDir(); err == nil {
					path = home + path[1:]
				}
			}
			m.err = ""
			remote := m.selectedRemote()
			return m, func() tea.Msg {
				return projectAddedMsg{name: name, path: path, remote: remote}
			}
		}
	}

	// Update the focused input.
	var cmd tea.Cmd
	switch m.focused {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		m.pathInput, cmd = m.pathInput.Update(msg)
		m.checkGitMode()
		if !m.isGitMode {
			suggCmd := m.updateSuggestions()
			return m, tea.Batch(cmd, suggCmd)
		}
	case 2:
		if m.isGitMode {
			m.clonePathInput, cmd = m.clonePathInput.Update(msg)
		}
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

	if m.isGitMode {
		b.WriteString(formLabelStyle.Render("Git URL") + "\n")
	} else {
		b.WriteString(formLabelStyle.Render("Path") + "\n")
	}
	b.WriteString(m.pathInput.View() + "\n")

	// Path suggestions (only in non-git mode)
	if !m.isGitMode && m.focused == 1 {
		if m.loadingSugg {
			b.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Render("    loading...") + "\n")
		} else if len(m.suggestions) > 0 {
			suggSelected := lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)
			suggNormal := lipgloss.NewStyle().
				Foreground(mutedColor)
			dirIndicator := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#60A5FA")) // blue for dirs

			for i, s := range m.suggestions {
				prefix := "    "
				if i == m.suggIdx {
					prefix = "  ▸ "
					if s.isDir {
						b.WriteString(suggSelected.Render(prefix) + dirIndicator.Render(s.display) + "\n")
					} else {
						b.WriteString(suggSelected.Render(prefix+s.display) + "\n")
					}
				} else {
					if s.isDir {
						b.WriteString(suggNormal.Render(prefix) + suggNormal.Render(s.display) + "\n")
					} else {
						b.WriteString(suggNormal.Render(prefix+s.display) + "\n")
					}
				}
			}
		}
	}
	b.WriteString("\n")

	// Clone path (only in git mode)
	if m.isGitMode {
		b.WriteString(formLabelStyle.Render("Clone Path") + "\n")
		b.WriteString(m.clonePathInput.View() + "\n\n")
	}

	// Remote selector (only shown if remotes exist)
	if len(m.remoteNames) > 0 {
		b.WriteString(formLabelStyle.Render("Remote") + "\n")

		remoteName := "local"
		if r := m.selectedRemote(); r != "" {
			remoteName = r
		}

		remoteField := m.remoteFieldIdx()
		if m.focused == remoteField {
			selector := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(primaryColor).
				Padding(0, 1).
				Render(fmt.Sprintf("◀ %s ▶", remoteName))
			b.WriteString(selector)
		} else {
			selector := lipgloss.NewStyle().
				Foreground(mutedColor).
				Render(fmt.Sprintf("  %s  ", remoteName))
			b.WriteString(selector)
		}
		b.WriteString("\n\n")
	}

	if m.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(dangerColor)
		b.WriteString(errStyle.Render(m.err) + "\n\n")
	}

	var hint string
	if m.isGitMode {
		hint = "Tab: switch fields · Enter: clone & add · Esc: cancel"
	} else {
		hint = "Tab: complete path / switch fields · ↑↓: navigate suggestions · Enter: add · Esc: cancel"
	}
	b.WriteString(formHintStyle.Render(hint))

	return b.String()
}
