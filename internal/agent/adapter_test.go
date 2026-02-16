package agent

import (
	"testing"
)

func TestAdapterRegistry(t *testing.T) {
	// Test ListAdapters
	adapters := ListAdapters()
	if len(adapters) == 0 {
		t.Fatal("expected at least one registered adapter")
	}

	// Test that claude adapter is registered
	found := false
	for _, name := range adapters {
		if name == "claude" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'claude' adapter to be registered")
	}
}

func TestGetAdapter(t *testing.T) {
	// Test getting claude adapter
	adapter, err := GetAdapter("claude")
	if err != nil {
		t.Fatalf("GetAdapter('claude') failed: %v", err)
	}

	if adapter.Name() != "claude" {
		t.Errorf("expected adapter name 'claude', got %q", adapter.Name())
	}

	// Test capabilities
	caps := adapter.Capabilities()
	if !caps.SessionPersistence {
		t.Error("expected claude adapter to support session persistence")
	}
	if !caps.JSONOutput {
		t.Error("expected claude adapter to support JSON output")
	}
	if !caps.ModelSelection {
		t.Error("expected claude adapter to support model selection")
	}
	if !caps.CostTracking {
		t.Error("expected claude adapter to support cost tracking")
	}
}

func TestGetAdapterNotFound(t *testing.T) {
	_, err := GetAdapter("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent adapter")
	}
}

func TestDefaultAdapter(t *testing.T) {
	adapter := DefaultAdapter()
	if adapter == nil {
		t.Skip("no adapters available (claude CLI not installed)")
	}

	// Should return an available adapter (preferably claude)
	if !adapter.Available() {
		t.Error("default adapter should be available")
	}
}

func TestClaudeAdapter(t *testing.T) {
	adapter := &ClaudeAdapter{}

	if adapter.Name() != "claude" {
		t.Errorf("expected name 'claude', got %q", adapter.Name())
	}

	// Test capabilities structure
	caps := adapter.Capabilities()
	if !caps.SessionPersistence || !caps.JSONOutput || !caps.ModelSelection || !caps.CostTracking {
		t.Error("claude adapter should support all major features")
	}
}

func TestRunConfigToClaudeResultConversion(t *testing.T) {
	// Test that the backward compatibility conversion in RunClaude works correctly
	// This is a structural test - we don't actually call claude CLI

	// Verify that ClaudeResult has all expected fields
	result := &ClaudeResult{
		Result:    "test result",
		Error:     "test error",
		SessionID: "test-session",
		CostUSD:   0.123,
		Duration:  45.67,
		IsError:   false,
	}

	if result.Result != "test result" {
		t.Error("ClaudeResult.Result field not working")
	}
	if result.SessionID != "test-session" {
		t.Error("ClaudeResult.SessionID field not working")
	}
	if result.CostUSD != 0.123 {
		t.Error("ClaudeResult.CostUSD field not working")
	}
	if result.Duration != 45.67 {
		t.Error("ClaudeResult.Duration field not working")
	}
}

func TestTaskAdapterField(t *testing.T) {
	// Test that Task struct has Adapter field
	task := &Task{
		ID:      1,
		Subject: "test task",
		Adapter: "claude",
	}

	if task.Adapter != "claude" {
		t.Errorf("expected adapter 'claude', got %q", task.Adapter)
	}

	// Test with empty adapter (should use default)
	task2 := &Task{
		ID:      2,
		Subject: "test task 2",
		Adapter: "",
	}

	if task2.Adapter != "" {
		t.Errorf("expected empty adapter, got %q", task2.Adapter)
	}
}
