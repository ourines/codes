package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHookRunner_Execute(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.json")

	// Create a script that reads stdin and writes to a file
	scriptPath := filepath.Join(tmpDir, "hook.sh")
	script := "#!/bin/sh\ncat > " + outputFile + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	runner := NewHookRunner(scriptPath)
	payload := HookPayload{
		Team:      "test-team",
		TaskID:    42,
		Subject:   "Build the widget",
		Status:    "completed",
		Agent:     "worker-1",
		Result:    "All tests passed",
		Timestamp: "2026-01-01T00:00:00Z",
	}

	if err := runner.Execute(payload); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	// Verify the payload was received
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var received HookPayload
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if received.Team != "test-team" {
		t.Errorf("Team = %q, want %q", received.Team, "test-team")
	}
	if received.TaskID != 42 {
		t.Errorf("TaskID = %d, want %d", received.TaskID, 42)
	}
	if received.Subject != "Build the widget" {
		t.Errorf("Subject = %q, want %q", received.Subject, "Build the widget")
	}
	if received.Status != "completed" {
		t.Errorf("Status = %q, want %q", received.Status, "completed")
	}
	if received.Agent != "worker-1" {
		t.Errorf("Agent = %q, want %q", received.Agent, "worker-1")
	}
	if received.Result != "All tests passed" {
		t.Errorf("Result = %q, want %q", received.Result, "All tests passed")
	}
}

func TestHookRunner_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "slow.sh")
	script := "#!/bin/sh\nsleep 60\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	runner := NewHookRunner(scriptPath)
	payload := HookPayload{
		Team:   "test-team",
		TaskID: 1,
		Status: "completed",
	}

	err := runner.Execute(payload)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !contains(err.Error(), "timed out") && !contains(err.Error(), "killed") {
		t.Errorf("expected timeout-related error, got: %v", err)
	}
}

func TestHookRunner_NonExistent(t *testing.T) {
	runner := NewHookRunner("/nonexistent/path/hook.sh")
	payload := HookPayload{
		Team:   "test-team",
		TaskID: 1,
		Status: "failed",
		Error:  "something broke",
	}

	err := runner.Execute(payload)
	if err == nil {
		t.Fatal("expected error for non-existent script, got nil")
	}
}

func TestHookRunner_ExitError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fail.sh")
	script := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	runner := NewHookRunner(scriptPath)
	payload := HookPayload{
		Team:   "test-team",
		TaskID: 1,
		Status: "failed",
		Error:  "build error",
	}

	err := runner.Execute(payload)
	if err == nil {
		t.Fatal("expected error for exit code 1, got nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
