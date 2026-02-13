package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"codes/internal/config"
)

// safeIDPattern matches only characters safe for file paths, shell scripts, and AppleScript.
var safeIDPattern = regexp.MustCompile(`[^a-zA-Z0-9_\-.]`)

// validEnvVarName matches valid environment variable names.
var validEnvVarName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// sanitizeID replaces unsafe characters in a session/project name with underscores.
func sanitizeID(name string) string {
	return safeIDPattern.ReplaceAllString(name, "_")
}

// Status represents the state of a session.
type Status int

const (
	StatusIdle    Status = iota // No active session
	StatusRunning               // Claude is running
	StatusExited                // Claude has exited
)

func (s Status) String() string {
	switch s {
	case StatusIdle:
		return "Idle"
	case StatusRunning:
		return "Running"
	case StatusExited:
		return "Exited"
	default:
		return "Unknown"
	}
}

// Session represents a single Claude Code session.
type Session struct {
	ID          string // unique ID, e.g. "myapp#1"
	ProjectName string
	ProjectPath string
	Status      Status
	PID         int
	StartedAt   time.Time

	mu sync.Mutex
}

// Uptime returns how long the session has been running.
func (s *Session) Uptime() time.Duration {
	if s.StartedAt.IsZero() {
		return 0
	}
	return time.Since(s.StartedAt).Truncate(time.Second)
}

// Manager manages multiple Claude Code sessions.
// Supports multiple sessions per project.
type Manager struct {
	sessions map[string]*Session // key = session ID
	counter  map[string]int      // key = project name, value = next counter
	mu       sync.RWMutex
	terminal string // terminal emulator preference
}

// NewManager creates a new session manager and restores any persisted sessions.
func NewManager(terminal string) *Manager {
	m := &Manager{
		sessions: make(map[string]*Session),
		counter:  make(map[string]int),
		terminal: terminal,
	}
	m.loadSessions()
	return m
}

// loadSessions scans the sessions directory and restores sessions whose processes are still alive.
func (m *Manager) loadSessions() {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		var ps persistedSession
		if err := json.Unmarshal(data, &ps); err != nil {
			continue
		}

		if ps.PID <= 0 || !isProcessAlive(ps.PID) {
			os.Remove(filepath.Join(dir, e.Name()))
			continue
		}

		s := &Session{
			ID:          ps.ID,
			ProjectName: ps.ProjectName,
			ProjectPath: ps.ProjectPath,
			Status:      StatusRunning,
			PID:         ps.PID,
			StartedAt:   ps.StartedAt,
		}
		m.sessions[ps.ID] = s

		// Update counter to avoid ID collisions
		// Session IDs are like "projectname-3", extract the number
		if idx := strings.LastIndex(ps.ID, "-"); idx >= 0 {
			if num, err := strconv.Atoi(ps.ID[idx+1:]); err == nil {
				projKey := ps.ID[:idx]
				if num >= m.counter[projKey] {
					m.counter[projKey] = num
				}
			}
		}

		// Monitor process exit in background
		go func(sess *Session) {
			for {
				time.Sleep(2 * time.Second)
				if !isProcessAlive(sess.PID) {
					sess.mu.Lock()
					sess.Status = StatusExited
					sess.mu.Unlock()
					os.Remove(pidFilePath(sess.ID))
					removeSessionFile(sess.ID)
					return
				}
			}
		}(s)
	}
}

// pidFilePath returns the path to the PID file for the given session ID.
func pidFilePath(sessionID string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("codes-session-%s.pid", sessionID))
}

// sessionsDirOverride allows tests to override the sessions directory.
var sessionsDirOverride string

// sessionsDir returns the directory for persisted session files (~/.codes/sessions/).
func sessionsDir() string {
	if sessionsDirOverride != "" {
		return sessionsDirOverride
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".codes", "sessions")
}

// sessionFilePath returns the path to the persisted session file for the given session ID.
func sessionFilePath(sessionID string) string {
	return filepath.Join(sessionsDir(), fmt.Sprintf("%s.json", sessionID))
}

// persistedSession is the on-disk representation of a session.
type persistedSession struct {
	ID          string    `json:"id"`
	ProjectName string    `json:"project_name"`
	ProjectPath string    `json:"project_path"`
	PID         int       `json:"pid"`
	StartedAt   time.Time `json:"started_at"`
}

// saveSession writes session metadata to disk.
func saveSession(s *Session) {
	dir := sessionsDir()
	os.MkdirAll(dir, 0755)

	ps := persistedSession{
		ID:          s.ID,
		ProjectName: s.ProjectName,
		ProjectPath: s.ProjectPath,
		PID:         s.PID,
		StartedAt:   s.StartedAt,
	}

	data, err := json.Marshal(ps)
	if err != nil {
		return
	}
	os.WriteFile(sessionFilePath(s.ID), data, 0644)
}

// removeSessionFile removes the persisted session file from disk.
func removeSessionFile(sessionID string) {
	os.Remove(sessionFilePath(sessionID))
}

// nextSessionID generates the next session ID for a project (e.g., "myapp-1", "myapp-2").
// The project name is sanitized to prevent injection in shell scripts and AppleScript.
func (m *Manager) nextSessionID(projectName string) string {
	m.counter[projectName]++
	return fmt.Sprintf("%s-%d", sanitizeID(projectName), m.counter[projectName])
}

// StartSession launches a new Claude Code session in a new terminal window.
// Always creates a new session (supports multiple per project).
func (m *Manager) StartSession(name, path string, args []string, env map[string]string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := m.nextSessionID(name)

	pid, err := openInTerminal(id, path, args, env, m.terminal)
	if err != nil {
		return nil, fmt.Errorf("failed to open terminal: %w", err)
	}

	s := &Session{
		ID:          id,
		ProjectName: name,
		ProjectPath: path,
		Status:      StatusRunning,
		PID:         pid,
		StartedAt:   time.Now(),
	}
	m.sessions[id] = s

	saveSession(s)

	// Monitor process exit in background
	go func() {
		for {
			time.Sleep(2 * time.Second)
			if !isProcessAlive(pid) {
				s.mu.Lock()
				s.Status = StatusExited
				s.mu.Unlock()
				os.Remove(pidFilePath(id))
				removeSessionFile(id)
				return
			}
		}
	}()

	return s, nil
}

// FocusSession brings the configured terminal to the foreground.
func (m *Manager) FocusSession() {
	focusTerminalWindow(m.terminal)
}

// GetSessionsByProject returns all sessions for a given project name.
func (m *Manager) GetSessionsByProject(name string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Session
	for _, s := range m.sessions {
		if s.ProjectName == name {
			result = append(result, s)
		}
	}
	return result
}

// GetRunningByProject returns running sessions for a given project.
func (m *Manager) GetRunningByProject(name string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Session
	for _, s := range m.sessions {
		if s.ProjectName == name && s.Status == StatusRunning {
			result = append(result, s)
		}
	}
	return result
}

// KillSession terminates a specific session by ID.
func (m *Manager) KillSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status != StatusRunning || s.PID <= 0 {
		s.Status = StatusExited
		return nil
	}

	err := killProcess(s.PID)
	s.Status = StatusExited
	os.Remove(pidFilePath(id))
	removeSessionFile(id)
	return err
}

// KillByProject terminates all sessions for a given project.
func (m *Manager) KillByProject(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.sessions {
		if s.ProjectName == name {
			s.mu.Lock()
			if s.Status == StatusRunning && s.PID > 0 {
				killProcess(s.PID)
				os.Remove(pidFilePath(s.ID))
				removeSessionFile(s.ID)
			}
			s.Status = StatusExited
			s.mu.Unlock()
		}
	}
}

// ListSessions returns all tracked sessions.
func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// RunningCount returns the number of currently running sessions.
func (m *Manager) RunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, s := range m.sessions {
		if s.Status == StatusRunning {
			count++
		}
	}
	return count
}

// RefreshStatus checks all running sessions and updates their status.
func (m *Manager) RefreshStatus() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sessions {
		s.mu.Lock()
		if s.Status == StatusRunning && s.PID > 0 && !isProcessAlive(s.PID) {
			s.Status = StatusExited
			os.Remove(pidFilePath(s.ID))
			removeSessionFile(s.ID)
		}
		s.mu.Unlock()
	}
}

// CleanExited removes exited sessions from tracking.
func (m *Manager) CleanExited() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, s := range m.sessions {
		if s.Status == StatusExited {
			removeSessionFile(id)
			delete(m.sessions, id)
		}
	}
}

// StartRemoteSession launches a Claude Code session on a remote host in a new terminal window.
func (m *Manager) StartRemoteSession(name string, host *config.RemoteHost, project string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := m.nextSessionID("remote-" + name)

	pid, err := openRemoteInTerminal(id, host, project, m.terminal)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote terminal: %w", err)
	}

	s := &Session{
		ID:          id,
		ProjectName: "remote:" + name,
		ProjectPath: host.UserAtHost(),
		Status:      StatusRunning,
		PID:         pid,
		StartedAt:   time.Now(),
	}
	m.sessions[id] = s

	saveSession(s)

	// Monitor process exit in background
	go func() {
		for {
			time.Sleep(2 * time.Second)
			if !isProcessAlive(pid) {
				s.mu.Lock()
				s.Status = StatusExited
				s.mu.Unlock()
				os.Remove(pidFilePath(id))
				removeSessionFile(id)
				return
			}
		}
	}()

	return s, nil
}
