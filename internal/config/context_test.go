package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinkedContextArgs_NoLinks(t *testing.T) {
	// Non-existent project should return nil
	args := LinkedContextArgs("nonexistent-project-xyz")
	if args != nil {
		t.Errorf("LinkedContextArgs(nonexistent) = %v, want nil", args)
	}
}

func TestGetLinkedProjectsSummary_WithLinks(t *testing.T) {
	// Setup: create a temp config with linked projects
	tmpDir := t.TempDir()
	linkedDir := filepath.Join(tmpDir, "linked-proj")
	os.MkdirAll(linkedDir, 0o755)

	// Write a CLAUDE.md in the linked project
	os.WriteFile(filepath.Join(linkedDir, "CLAUDE.md"), []byte("# Linked Project\nThis is a test project."), 0o644)

	cfg := &Config{
		Projects: map[string]ProjectEntry{
			"main-proj": {
				Path: tmpDir,
				Links: []ProjectLink{
					{Name: "linked-proj", Role: "backend"},
				},
			},
			"linked-proj": {
				Path: linkedDir,
			},
		},
	}

	// Override config loading for this test
	origLoad := loadConfigFunc
	loadConfigFunc = func() (*Config, error) { return cfg, nil }
	defer func() { loadConfigFunc = origLoad }()

	summary, err := GetLinkedProjectsSummary("main-proj")
	if err != nil {
		t.Fatalf("GetLinkedProjectsSummary: %v", err)
	}
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}

	// Should contain project info
	if !contains(summary, "linked-proj") {
		t.Error("summary should contain linked project name")
	}
	if !contains(summary, "backend") {
		t.Error("summary should contain role")
	}
	if !contains(summary, "Linked Project") {
		t.Error("summary should contain CLAUDE.md content")
	}
}

func TestLinkedContextArgs_WithLinks(t *testing.T) {
	tmpDir := t.TempDir()
	linkedDir := filepath.Join(tmpDir, "lib")
	os.MkdirAll(linkedDir, 0o755)

	cfg := &Config{
		Projects: map[string]ProjectEntry{
			"app": {
				Path:  tmpDir,
				Links: []ProjectLink{{Name: "lib", Role: "shared library"}},
			},
			"lib": {Path: linkedDir},
		},
	}

	origLoad := loadConfigFunc
	loadConfigFunc = func() (*Config, error) { return cfg, nil }
	defer func() { loadConfigFunc = origLoad }()

	args := LinkedContextArgs("app")
	if len(args) != 2 {
		t.Fatalf("LinkedContextArgs(app) = %v, want 2 args", args)
	}
	if args[0] != "--append-system-prompt" {
		t.Errorf("args[0] = %q, want --append-system-prompt", args[0])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
