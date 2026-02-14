package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestDir creates a temporary teams directory and overrides teamsBaseDir.
func setupTestDir(t *testing.T) (cleanup func()) {
	t.Helper()
	tmpDir := t.TempDir()

	origFunc := teamsBaseDirFunc
	teamsBaseDirFunc = func() string { return tmpDir }

	return func() {
		teamsBaseDirFunc = origFunc
	}
}

func TestCreateAndGetTeam(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	cfg, err := CreateTeam("test-team", "A test team", "/tmp/work")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	if cfg.Name != "test-team" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-team")
	}
	if cfg.Description != "A test team" {
		t.Errorf("Description = %q, want %q", cfg.Description, "A test team")
	}

	// Verify directory structure
	for _, sub := range []string{"tasks", "messages", "agents"} {
		dir := filepath.Join(teamsBaseDirFunc(), "test-team", sub)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", dir)
		}
	}

	// Get team
	got, err := GetTeam("test-team")
	if err != nil {
		t.Fatalf("GetTeam: %v", err)
	}
	if got.Name != "test-team" {
		t.Errorf("GetTeam Name = %q, want %q", got.Name, "test-team")
	}

	// Duplicate should fail
	_, err = CreateTeam("test-team", "", "")
	if err == nil {
		t.Error("CreateTeam duplicate should fail")
	}
}

func TestListTeams(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("alpha", "", "")
	CreateTeam("beta", "", "")

	teams, err := ListTeams()
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	if len(teams) != 2 {
		t.Errorf("ListTeams = %d teams, want 2", len(teams))
	}
}

func TestDeleteTeam(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("to-delete", "", "")
	if err := DeleteTeam("to-delete"); err != nil {
		t.Fatalf("DeleteTeam: %v", err)
	}

	_, err := GetTeam("to-delete")
	if err == nil {
		t.Error("GetTeam should fail after deletion")
	}
}

func TestAddRemoveMember(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("team1", "", "")

	member := TeamMember{Name: "worker1", Role: "coder", Model: "sonnet"}
	if err := AddMember("team1", member); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	cfg, _ := GetTeam("team1")
	if len(cfg.Members) != 1 {
		t.Fatalf("Members = %d, want 1", len(cfg.Members))
	}
	if cfg.Members[0].Name != "worker1" {
		t.Errorf("Member name = %q, want %q", cfg.Members[0].Name, "worker1")
	}

	// Duplicate should fail
	err := AddMember("team1", member)
	if err == nil {
		t.Error("AddMember duplicate should fail")
	}

	// Remove
	if err := RemoveMember("team1", "worker1"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	cfg, _ = GetTeam("team1")
	if len(cfg.Members) != 0 {
		t.Errorf("Members after remove = %d, want 0", len(cfg.Members))
	}
}

func TestTaskCRUD(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("task-team", "", "")

	// Create tasks
	t1, err := CreateTask("task-team", "First task", "do something", "", nil, "")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if t1.ID != 1 {
		t.Errorf("Task ID = %d, want 1", t1.ID)
	}
	if t1.Status != TaskPending {
		t.Errorf("Status = %s, want %s", t1.Status, TaskPending)
	}
	if t1.Priority != PriorityNormal {
		t.Errorf("Priority = %s, want %s (default)", t1.Priority, PriorityNormal)
	}

	t2, err := CreateTask("task-team", "Second task", "", "worker1", nil, "")
	if err != nil {
		t.Fatalf("CreateTask 2: %v", err)
	}
	if t2.Status != TaskAssigned {
		t.Errorf("Status = %s, want %s (auto-assigned)", t2.Status, TaskAssigned)
	}

	// List tasks
	tasks, err := ListTasks("task-team", "", "")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("ListTasks = %d, want 2", len(tasks))
	}

	// Filter by status
	tasks, _ = ListTasks("task-team", TaskPending, "")
	if len(tasks) != 1 {
		t.Errorf("ListTasks(pending) = %d, want 1", len(tasks))
	}

	// Get task
	got, err := GetTask("task-team", 1)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Subject != "First task" {
		t.Errorf("Subject = %q, want %q", got.Subject, "First task")
	}

	// Assign task
	assigned, err := AssignTask("task-team", 1, "worker1")
	if err != nil {
		t.Fatalf("AssignTask: %v", err)
	}
	if assigned.Status != TaskAssigned {
		t.Errorf("Status after assign = %s, want %s", assigned.Status, TaskAssigned)
	}

	// Complete task
	completed, err := CompleteTask("task-team", 1, "done!")
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if completed.Status != TaskCompleted {
		t.Errorf("Status after complete = %s, want %s", completed.Status, TaskCompleted)
	}
	if completed.Result != "done!" {
		t.Errorf("Result = %q, want %q", completed.Result, "done!")
	}

	// Fail task
	failed, err := FailTask("task-team", 2, "oops")
	if err != nil {
		t.Fatalf("FailTask: %v", err)
	}
	if failed.Status != TaskFailed {
		t.Errorf("Status after fail = %s, want %s", failed.Status, TaskFailed)
	}

	// Cancel - should fail on completed task
	_, err = CancelTask("task-team", 1)
	if err == nil {
		t.Error("CancelTask on completed should fail")
	}
}

func TestTaskBlocking(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("block-team", "", "")

	t1, _ := CreateTask("block-team", "Dep task", "", "", nil, "")
	t2, _ := CreateTask("block-team", "Blocked task", "", "", []int{t1.ID}, "")

	blocked, err := IsTaskBlocked("block-team", t2)
	if err != nil {
		t.Fatalf("IsTaskBlocked: %v", err)
	}
	if !blocked {
		t.Error("Task should be blocked")
	}

	// Complete dependency
	AssignTask("block-team", t1.ID, "w")
	CompleteTask("block-team", t1.ID, "done")

	t2, _ = GetTask("block-team", t2.ID)
	blocked, err = IsTaskBlocked("block-team", t2)
	if err != nil {
		t.Fatalf("IsTaskBlocked after complete: %v", err)
	}
	if blocked {
		t.Error("Task should not be blocked after dependency completes")
	}
}

func TestMessages(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("msg-team", "", "")

	// Send messages
	m1, err := SendMessage("msg-team", "alice", "bob", "hello bob")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if m1.From != "alice" || m1.To != "bob" {
		t.Errorf("Message from=%s to=%s", m1.From, m1.To)
	}

	SendMessage("msg-team", "charlie", "bob", "hey bob")
	BroadcastMessage("msg-team", "leader", "attention all")

	// Get messages for bob
	msgs, err := GetMessages("msg-team", "bob", false)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	// bob should see: 2 direct + 1 broadcast = 3
	if len(msgs) != 3 {
		t.Errorf("GetMessages(bob) = %d, want 3", len(msgs))
	}

	// Get unread only
	msgs, _ = GetMessages("msg-team", "bob", true)
	if len(msgs) != 3 {
		t.Errorf("Unread messages = %d, want 3", len(msgs))
	}

	// Mark read
	MarkRead("msg-team", m1.ID)
	msgs, _ = GetMessages("msg-team", "bob", true)
	if len(msgs) != 2 {
		t.Errorf("After marking read, unread = %d, want 2", len(msgs))
	}
}

func TestAgentState(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("state-team", "", "")

	state := &AgentState{
		Name:   "worker1",
		Team:   "state-team",
		PID:    12345,
		Status: AgentRunning,
	}

	if err := SaveAgentState(state); err != nil {
		t.Fatalf("SaveAgentState: %v", err)
	}

	got, err := GetAgentState("state-team", "worker1")
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if got == nil {
		t.Fatal("GetAgentState returned nil")
	}
	if got.PID != 12345 {
		t.Errorf("PID = %d, want 12345", got.PID)
	}
	if got.Status != AgentRunning {
		t.Errorf("Status = %s, want %s", got.Status, AgentRunning)
	}
}

func TestMessageTypes(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("type-team", "", "")

	// Send chat message
	m1, err := SendMessage("type-team", "alice", "bob", "hello")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if m1.Type != MsgChat {
		t.Errorf("SendMessage type = %s, want %s", m1.Type, MsgChat)
	}

	// Send task report
	m2, err := SendTaskReport("type-team", "worker1", "", MsgTaskCompleted, 42, "Task done")
	if err != nil {
		t.Fatalf("SendTaskReport: %v", err)
	}
	if m2.Type != MsgTaskCompleted {
		t.Errorf("Report type = %s, want %s", m2.Type, MsgTaskCompleted)
	}
	if m2.TaskID != 42 {
		t.Errorf("TaskID = %d, want 42", m2.TaskID)
	}

	// Filter by type
	reports, err := GetMessagesByType("type-team", "bob", MsgTaskCompleted, false)
	if err != nil {
		t.Fatalf("GetMessagesByType: %v", err)
	}
	// bob sees broadcast task_completed (m2 has to="" which is broadcast)
	if len(reports) != 1 {
		t.Errorf("GetMessagesByType(task_completed) = %d, want 1", len(reports))
	}

	chats, _ := GetMessagesByType("type-team", "bob", MsgChat, false)
	if len(chats) != 1 {
		t.Errorf("GetMessagesByType(chat) = %d, want 1", len(chats))
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == id2 {
		t.Error("generateID should produce unique IDs")
	}
	if len(id1) != 36 {
		t.Errorf("generateID length = %d, want 36 (UUID format)", len(id1))
	}
}

func TestTaskPriority(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("prio-team", "", "")

	// Create tasks with different priorities
	CreateTask("prio-team", "Low priority", "", "", nil, PriorityLow)
	CreateTask("prio-team", "Normal priority", "", "", nil, PriorityNormal)
	CreateTask("prio-team", "High priority", "", "", nil, PriorityHigh)
	CreateTask("prio-team", "Default priority", "", "", nil, "")

	tasks, err := ListTasks("prio-team", "", "")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 4 {
		t.Fatalf("ListTasks = %d, want 4", len(tasks))
	}

	// High priority should be first
	if tasks[0].Priority != PriorityHigh {
		t.Errorf("First task priority = %s, want %s", tasks[0].Priority, PriorityHigh)
	}
	// Low priority should be last
	if tasks[len(tasks)-1].Priority != PriorityLow {
		t.Errorf("Last task priority = %s, want %s", tasks[len(tasks)-1].Priority, PriorityLow)
	}
}

func TestIsAgentAlive(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("alive-team", "", "")

	// No state → not alive
	alive := IsAgentAlive("alive-team", "worker1")
	if alive {
		t.Error("IsAgentAlive should return false with no state")
	}

	// State with PID 0 → not alive
	SaveAgentState(&AgentState{
		Name:   "worker1",
		Team:   "alive-team",
		PID:    0,
		Status: AgentRunning,
	})
	alive = IsAgentAlive("alive-team", "worker1")
	if alive {
		t.Error("IsAgentAlive should return false with PID 0")
	}

	// State with a dead PID → not alive, status updated to stopped
	SaveAgentState(&AgentState{
		Name:   "worker1",
		Team:   "alive-team",
		PID:    999999, // unlikely to be a real process
		Status: AgentRunning,
	})
	alive = IsAgentAlive("alive-team", "worker1")
	if alive {
		t.Error("IsAgentAlive should return false for dead process")
	}

	// Verify status was updated to stopped
	state, _ := GetAgentState("alive-team", "worker1")
	if state != nil && state.Status != AgentStopped {
		t.Errorf("Status after dead check = %s, want %s", state.Status, AgentStopped)
	}

	// State with our own PID → alive
	SaveAgentState(&AgentState{
		Name:   "worker2",
		Team:   "alive-team",
		PID:    os.Getpid(),
		Status: AgentRunning,
	})
	alive = IsAgentAlive("alive-team", "worker2")
	if !alive {
		t.Error("IsAgentAlive should return true for current process")
	}
}

func TestTaskDefaultPriority(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("default-prio", "", "")

	task, err := CreateTask("default-prio", "Test", "", "", nil, "")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.Priority != PriorityNormal {
		t.Errorf("Default priority = %s, want %s", task.Priority, PriorityNormal)
	}

	task2, err := CreateTask("default-prio", "High", "", "", nil, PriorityHigh)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task2.Priority != PriorityHigh {
		t.Errorf("Explicit priority = %s, want %s", task2.Priority, PriorityHigh)
	}
}
