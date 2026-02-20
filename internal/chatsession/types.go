package chatsession

import (
	"encoding/json"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SessionStatus represents the state of a chat session.
type SessionStatus string

const (
	StatusCreating SessionStatus = "creating"
	StatusReady    SessionStatus = "ready"  // Waiting for user input
	StatusBusy     SessionStatus = "busy"   // Claude is processing
	StatusClosed   SessionStatus = "closed"
)

// ChatSession represents an interactive Claude Code session with WebSocket support.
type ChatSession struct {
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

	mu       sync.Mutex
	process  *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	clients  map[*websocket.Conn]bool
	messages []json.RawMessage // Cached messages for reconnection replay
	done     chan struct{}      // Closed when readPump exits
}

// SessionManager is a thread-safe registry of active chat sessions.
type SessionManager struct {
	sessions map[string]*ChatSession
	mu       sync.RWMutex
}

// --- Stdin message types (sent to Claude) ---

// stdinUserMessage wraps a user message for Claude's stream-json stdin.
type stdinUserMessage struct {
	Type    string          `json:"type"`
	Message stdinMsgContent `json:"message"`
}

type stdinMsgContent struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// stdinControlRequest sends a control signal (e.g., interrupt) to Claude.
type stdinControlRequest struct {
	Type      string              `json:"type"`
	RequestID string              `json:"request_id"`
	Request   stdinControlPayload `json:"request"`
}

type stdinControlPayload struct {
	Subtype string `json:"subtype"`
}

// stdinControlResponse sends a permission response to Claude.
type stdinControlResponse struct {
	Type     string                    `json:"type"`
	Response stdinControlResponseInner `json:"response"`
}

type stdinControlResponseInner struct {
	Subtype   string                       `json:"subtype"`
	RequestID string                       `json:"request_id"`
	Response  stdinControlResponseBehavior `json:"response"`
}

type stdinControlResponseBehavior struct {
	Behavior     string      `json:"behavior"` // "allow" or "deny"
	UpdatedInput any `json:"updatedInput,omitempty"`
	Message      string      `json:"message,omitempty"`
}

// --- WebSocket message types (client <-> server) ---

// wsIncoming represents a message from a WebSocket client.
type wsIncoming struct {
	Type         string          `json:"type"`                    // user_message, interrupt, permission_response
	Content      string          `json:"content,omitempty"`       // For user_message
	RequestID    string          `json:"request_id,omitempty"`    // For permission_response
	Allow        bool            `json:"allow,omitempty"`         // For permission_response
	UpdatedInput json.RawMessage `json:"updated_input,omitempty"` // For permission_response
}

// wsOutgoing represents a message sent to WebSocket clients.
type wsOutgoing struct {
	Type    string          `json:"type"`              // claude_event, session_status, error
	Event   json.RawMessage `json:"event,omitempty"`   // Raw Claude stream-json event
	Status  SessionStatus   `json:"status,omitempty"`  // For session_status
	Message string          `json:"message,omitempty"` // For error
}
