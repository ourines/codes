package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "myproject", "myproject"},
		{"with spaces", "my project", "my_project"},
		{"with special chars", "my@project#1", "my_project_1"},
		{"with dots and dashes", "my-project.v2", "my-project.v2"},
		{"with slashes", "path/to/project", "path_to_project"},
		{"empty string", "", ""},
		{"unicode chars", "项目名", "___"},
		{"already safe", "test_session-1.0", "test_session-1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeID(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPidFilePath(t *testing.T) {
	result := pidFilePath("test-session")
	expected := filepath.Join(os.TempDir(), "codes-session-test-session.pid")
	if result != expected {
		t.Errorf("pidFilePath(%q) = %q, want %q", "test-session", result, expected)
	}
}

func TestValidEnvVarName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		isValid bool
	}{
		{"simple var", "HOME", true},
		{"with underscore", "MY_VAR", true},
		{"starts with underscore", "_PRIVATE", true},
		{"with digits", "VAR123", true},
		{"underscore and digits", "_V2_TEST", true},
		{"starts with digit", "1VAR", false},
		{"with dash", "MY-VAR", false},
		{"with space", "MY VAR", false},
		{"with dot", "MY.VAR", false},
		{"empty string", "", false},
		{"with equals", "VAR=value", false},
		{"with special char", "VAR$NAME", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validEnvVarName.MatchString(tt.input)
			if result != tt.isValid {
				t.Errorf("validEnvVarName.MatchString(%q) = %v, want %v", tt.input, result, tt.isValid)
			}
		})
	}
}

func TestSessionPersistence(t *testing.T) {
	// Use a temp directory as the sessions dir
	tmpDir := t.TempDir()
	sessionsDirOverride = tmpDir
	t.Cleanup(func() { sessionsDirOverride = "" })

	s := &Session{
		ID:          "testproj-1",
		ProjectName: "testproj",
		ProjectPath: "/tmp/testproj",
		PID:         os.Getpid(), // current process, guaranteed alive
		StartedAt:   time.Now().Truncate(time.Second),
	}

	// Test saveSession
	saveSession(s)
	fp := filepath.Join(tmpDir, "testproj-1.json")
	if _, err := os.Stat(fp); err != nil {
		t.Fatalf("session file not created: %v", err)
	}

	// Verify file content
	data, _ := os.ReadFile(fp)
	var ps persistedSession
	if err := json.Unmarshal(data, &ps); err != nil {
		t.Fatalf("invalid session JSON: %v", err)
	}
	if ps.ID != s.ID || ps.ProjectName != s.ProjectName || ps.PID != s.PID {
		t.Errorf("persisted data mismatch: got %+v", ps)
	}

	// Test loadSessions restores alive session
	m := &Manager{
		sessions: make(map[string]*Session),
		counter:  make(map[string]int),
	}
	m.loadSessions()

	if len(m.sessions) != 1 {
		t.Fatalf("expected 1 restored session, got %d", len(m.sessions))
	}
	restored := m.sessions["testproj-1"]
	if restored == nil {
		t.Fatal("session testproj-1 not restored")
	}
	if restored.Status != StatusRunning {
		t.Errorf("expected StatusRunning, got %v", restored.Status)
	}
	if restored.ProjectName != "testproj" {
		t.Errorf("expected project name testproj, got %s", restored.ProjectName)
	}

	// Counter should be updated to avoid collisions
	if m.counter["testproj"] != 1 {
		t.Errorf("expected counter=1 for testproj, got %d", m.counter["testproj"])
	}

	// Test removeSessionFile
	removeSessionFile("testproj-1")
	if _, err := os.Stat(fp); !os.IsNotExist(err) {
		t.Error("session file should be removed")
	}
}

func TestLoadSessions_DeadProcess(t *testing.T) {
	tmpDir := t.TempDir()
	sessionsDirOverride = tmpDir
	t.Cleanup(func() { sessionsDirOverride = "" })

	// Write a session file with a PID that doesn't exist
	ps := persistedSession{
		ID:          "dead-1",
		ProjectName: "dead",
		ProjectPath: "/tmp/dead",
		PID:         999999999, // unlikely to be alive
		StartedAt:   time.Now(),
	}
	data, _ := json.Marshal(ps)
	os.WriteFile(filepath.Join(tmpDir, "dead-1.json"), data, 0644)

	m := &Manager{
		sessions: make(map[string]*Session),
		counter:  make(map[string]int),
	}
	m.loadSessions()

	if len(m.sessions) != 0 {
		t.Errorf("dead session should not be restored, got %d sessions", len(m.sessions))
	}

	// File should be cleaned up
	if _, err := os.Stat(filepath.Join(tmpDir, "dead-1.json")); !os.IsNotExist(err) {
		t.Error("dead session file should be removed")
	}
}
