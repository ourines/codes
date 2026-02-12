package session

import (
	"os"
	"path/filepath"
	"testing"
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
