package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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

// NewManager creates a new session manager.
func NewManager(terminal string) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		counter:  make(map[string]int),
		terminal: terminal,
	}
}

// pidFilePath returns the path to the PID file for the given session ID.
func pidFilePath(sessionID string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("codes-session-%s.pid", sessionID))
}

// nextSessionID generates the next session ID for a project (e.g., "myapp#1", "myapp#2").
func (m *Manager) nextSessionID(projectName string) string {
	m.counter[projectName]++
	return fmt.Sprintf("%s#%d", projectName, m.counter[projectName])
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

	// Monitor process exit in background
	go func() {
		for {
			time.Sleep(2 * time.Second)
			if !isProcessAlive(pid) {
				s.mu.Lock()
				s.Status = StatusExited
				s.mu.Unlock()
				os.Remove(pidFilePath(id))
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
			delete(m.sessions, id)
		}
	}
}
