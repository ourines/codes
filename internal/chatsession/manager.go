package chatsession

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DefaultManager is the global session registry.
var DefaultManager = NewSessionManager()

// NewSessionManager creates an empty SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*ChatSession),
	}
}

// Create allocates a new ChatSession in "creating" state.
// The caller must call session.Start(firstMessage) or session.Resume(id) to activate it.
func (m *SessionManager) Create(projectName, projectPath, model string) (*ChatSession, error) {
	if projectPath == "" {
		return nil, fmt.Errorf("projectPath is required")
	}

	id := generateID()

	session := &ChatSession{
		ID:           id,
		ProjectName:  projectName,
		ProjectPath:  projectPath,
		Model:        model,
		Status:       StatusCreating,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		clients:      make(map[*websocket.Conn]bool),
	}

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()

	return session, nil
}

// Get returns a session by ID, or false if not found.
func (m *SessionManager) Get(id string) (*ChatSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// List returns all active sessions.
func (m *SessionManager) List() []*ChatSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ChatSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// Delete closes and removes a session.
func (m *SessionManager) Delete(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session %s not found", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	return s.Close()
}

// Resume creates a new ChatSession that resumes a previous Claude session.
func (m *SessionManager) Resume(claudeSessionID, projectName, projectPath, model string) (*ChatSession, error) {
	session, err := m.Create(projectName, projectPath, model)
	if err != nil {
		return nil, err
	}

	if err := session.Resume(claudeSessionID); err != nil {
		// Clean up on failure.
		m.mu.Lock()
		delete(m.sessions, session.ID)
		m.mu.Unlock()
		return nil, err
	}

	return session, nil
}

// generateID produces a unique session identifier.
func generateID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("cs-%d-%x", time.Now().UnixNano(), b)
}

// Snapshot returns a read-only snapshot of a session's public state.
// This avoids exposing the mutex to callers.
func (s *ChatSession) Snapshot() SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SessionInfo{
		ID:              s.ID,
		ProjectName:     s.ProjectName,
		ProjectPath:     s.ProjectPath,
		Model:           s.Model,
		ClaudeSessionID: s.ClaudeSessionID,
		Status:          s.Status,
		CreatedAt:       s.CreatedAt,
		LastActiveAt:    s.LastActiveAt,
		CostUSD:         s.CostUSD,
		TurnCount:       s.TurnCount,
		ClientCount:     len(s.clients),
	}
}

// SessionInfo is a read-only view of a ChatSession (safe to serialize).
type SessionInfo struct {
	ID              string        `json:"id"`
	ProjectName     string        `json:"projectName,omitempty"`
	ProjectPath     string        `json:"projectPath"`
	Model           string        `json:"model,omitempty"`
	ClaudeSessionID string        `json:"claudeSessionId,omitempty"`
	Status          SessionStatus `json:"status"`
	CreatedAt       time.Time     `json:"createdAt"`
	LastActiveAt    time.Time     `json:"lastActiveAt"`
	CostUSD         float64       `json:"costUsd"`
	TurnCount       int           `json:"turnCount"`
	ClientCount     int           `json:"clientCount"`
}

// CloseAll shuts down every session. Used during server shutdown.
func (m *SessionManager) CloseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(sid string) {
			defer wg.Done()
			m.Delete(sid)
		}(id)
	}
	wg.Wait()
}
