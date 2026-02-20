package assistant

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// Session holds the conversation history for a single user/chat.
type Session struct {
	ID       string
	Messages []anthropic.BetaMessageParam
}

func sessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".codes", "assistant")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func sanitizeSessionID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	s := b.String()
	if s == "" {
		return "default"
	}
	return s
}

// LoadSession loads a session from disk. Returns an empty session if not found.
func LoadSession(id string) (*Session, error) {
	dir, err := sessionsDir()
	if err != nil {
		return nil, err
	}
	safe := sanitizeSessionID(id)
	path := filepath.Join(dir, safe+".json")

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Session{ID: id}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}

	// Sessions are stored as raw JSON array of message params.
	// We store them as []json.RawMessage to avoid SDK struct versioning issues.
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		// Corrupted session — start fresh.
		return &Session{ID: id}, nil
	}

	msgs := make([]anthropic.BetaMessageParam, 0, len(raw))
	for _, r := range raw {
		var m anthropic.BetaMessageParam
		if err := json.Unmarshal(r, &m); err == nil {
			msgs = append(msgs, m)
		}
	}
	return &Session{ID: id, Messages: msgs}, nil
}

// Save persists the session to disk.
func (s *Session) Save() error {
	dir, err := sessionsDir()
	if err != nil {
		return err
	}
	safe := sanitizeSessionID(s.ID)
	path := filepath.Join(dir, safe+".json")

	data, err := json.Marshal(s.Messages)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return os.Rename(tmp, path)
}

// Clear deletes the session file.
func ClearSession(id string) error {
	dir, err := sessionsDir()
	if err != nil {
		return err
	}
	safe := sanitizeSessionID(id)
	path := filepath.Join(dir, safe+".json")
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
