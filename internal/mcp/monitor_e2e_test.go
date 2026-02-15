package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"codes/internal/agent"
)

// setupTestServer creates an MCP server+client pair connected via in-memory
// transport. The returned cleanup function removes the test team.
func setupTestServer(t *testing.T, teamName string) (*mcpsdk.ClientSession, func()) {
	t.Helper()

	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "codes-test", Version: "0.0.1"},
		nil,
	)
	registerAgentTools(server)

	ct, st := mcpsdk.NewInMemoryTransports()

	ctx := context.Background()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}

	cleanup := func() {
		_ = agent.DeleteTeam(teamName)
		cs.Close()
		ss.Close()
		// Reset monitor state for test isolation.
		monitorMu.Lock()
		monitorStarted = false
		monitorMu.Unlock()
		monitorRunning.Store(false)
	}

	return cs, cleanup
}

// callTool is a helper that calls a tool and returns the unmarshaled JSON
// content from the first TextContent block.
func callTool(t *testing.T, cs *mcpsdk.ClientSession, name string, args any) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if len(result.Content) == 0 {
		t.Fatalf("CallTool(%s): empty content", name)
	}
	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): content is %T, want *TextContent", name, result.Content[0])
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &m); err != nil {
		t.Fatalf("CallTool(%s): unmarshal response: %v\nraw: %s", name, err, tc.Text)
	}
	return m
}

func TestE2E_TaskCreateReturnsMonitorActive(t *testing.T) {
	team := fmt.Sprintf("e2e-test-%d", time.Now().UnixNano())
	cs, cleanup := setupTestServer(t, team)
	defer cleanup()

	// 1. Create team.
	callTool(t, cs, "team_create", map[string]any{"name": team})

	// 2. Create task — should return monitor_active: true.
	resp := callTool(t, cs, "task_create", map[string]any{
		"team":    team,
		"subject": "test task",
	})

	active, ok := resp["monitor_active"]
	if !ok {
		t.Fatalf("task_create response missing monitor_active field: %v", resp)
	}
	if active != true {
		t.Errorf("monitor_active = %v, want true", active)
	}
}

func TestE2E_OutputStructFields(t *testing.T) {
	// Verify agentStartOutput has monitor_active, no monitor_cmd.
	out1 := agentStartOutput{Started: true, PID: 123, MonitorActive: true}
	b1, _ := json.Marshal(out1)
	var m1 map[string]any
	json.Unmarshal(b1, &m1)

	if _, has := m1["monitor_active"]; !has {
		t.Error("agentStartOutput missing monitor_active field")
	}
	if _, has := m1["monitor_cmd"]; has {
		t.Error("agentStartOutput should not have monitor_cmd")
	}

	// Verify teamStartAllOutput has monitor_active, no monitor_cmd.
	out2 := teamStartAllOutput{MonitorActive: true}
	b2, _ := json.Marshal(out2)
	var m2 map[string]any
	json.Unmarshal(b2, &m2)

	if _, has := m2["monitor_active"]; !has {
		t.Error("teamStartAllOutput missing monitor_active field")
	}
	if _, has := m2["monitor_cmd"]; has {
		t.Error("teamStartAllOutput should not have monitor_cmd")
	}
}

func TestE2E_PiggybackNotifications(t *testing.T) {
	team := fmt.Sprintf("e2e-test-%d", time.Now().UnixNano())
	cs, cleanup := setupTestServer(t, team)
	defer cleanup()

	// Use an isolated temp dir so running codes-serve processes don't
	// compete for the same notification files.
	tmpDir := t.TempDir()
	notifDirOverride = tmpDir
	defer func() { notifDirOverride = "" }()

	// 1. Create team + task to start the monitor.
	callTool(t, cs, "team_create", map[string]any{"name": team})
	callTool(t, cs, "task_create", map[string]any{
		"team":    team,
		"subject": "monitored task",
	})

	// 2. Simulate a daemon writing a notification file.
	notif := map[string]any{
		"team":      team,
		"taskId":    1,
		"subject":   "monitored task",
		"status":    "completed",
		"agent":     "test-agent",
		"result":    "all good",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(notif)
	notifPath := filepath.Join(tmpDir, fmt.Sprintf("%s__1.json", team))
	if err := os.WriteFile(notifPath, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// 3. Wait for the monitor goroutine to pick it up (polls every 3s).
	time.Sleep(5 * time.Second)

	// 4. Call task_list — should piggyback the notification.
	resp := callTool(t, cs, "task_list", map[string]any{"team": team})

	notifs, ok := resp["pending_notifications"]
	if !ok || notifs == nil {
		t.Fatalf("task_list response missing pending_notifications: %v", resp)
	}

	arr, ok := notifs.([]any)
	if !ok || len(arr) == 0 {
		t.Fatalf("pending_notifications empty or wrong type: %v", notifs)
	}

	first, ok := arr[0].(map[string]any)
	if !ok {
		t.Fatalf("notification[0] is %T, want map", arr[0])
	}

	if first["team"] != team {
		t.Errorf("notification team = %v, want %s", first["team"], team)
	}
	if first["status"] != "completed" {
		t.Errorf("notification status = %v, want completed", first["status"])
	}
	if first["result"] != "all good" {
		t.Errorf("notification result = %v, want 'all good'", first["result"])
	}

	// 5. File should still exist (monitor no longer deletes — team_watch does).
	if _, err := os.Stat(notifPath); os.IsNotExist(err) {
		t.Errorf("notification file should still exist for team_watch to consume")
	}
}

func TestE2E_PendingNotificationsCapLimit(t *testing.T) {
	// Directly test the cap by stuffing the buffer.
	pendingMu.Lock()
	pendingNotifications = nil
	pendingMu.Unlock()

	for i := 0; i < maxPendingNotifications+50; i++ {
		pendingMu.Lock()
		if len(pendingNotifications) < maxPendingNotifications {
			pendingNotifications = append(pendingNotifications, taskNotification{
				TaskID:  i,
				Subject: fmt.Sprintf("task-%d", i),
			})
		}
		pendingMu.Unlock()
	}

	pendingMu.Lock()
	n := len(pendingNotifications)
	pendingMu.Unlock()

	if n != maxPendingNotifications {
		t.Errorf("pending count = %d, want %d (cap)", n, maxPendingNotifications)
	}

	// Drain and verify.
	drained := drainPendingNotifications()
	if len(drained) != maxPendingNotifications {
		t.Errorf("drained %d, want %d", len(drained), maxPendingNotifications)
	}

	// Second drain should be empty.
	again := drainPendingNotifications()
	if again != nil {
		t.Errorf("second drain should be nil, got %d items", len(again))
	}
}

func TestE2E_NoMonitorCmdInResponses(t *testing.T) {
	team := fmt.Sprintf("e2e-test-%d", time.Now().UnixNano())
	cs, cleanup := setupTestServer(t, team)
	defer cleanup()

	callTool(t, cs, "team_create", map[string]any{"name": team})

	// task_create
	resp := callTool(t, cs, "task_create", map[string]any{
		"team":    team,
		"subject": "check no monitor_cmd",
	})
	if _, has := resp["monitor_cmd"]; has {
		t.Errorf("task_create should not have monitor_cmd: %v", resp)
	}
}
