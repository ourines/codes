package agent

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
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

// newTestLogger returns a logger that writes to stderr for test output.
func newTestLogger() *log.Logger {
	return log.New(os.Stderr, "[test] ", log.LstdFlags)
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
	t1, err := CreateTask("task-team", "First task", "do something", "", nil, "", "", "")
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

	t2, err := CreateTask("task-team", "Second task", "", "worker1", nil, "", "", "")
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

	t1, _ := CreateTask("block-team", "Dep task", "", "", nil, "", "", "")
	t2, _ := CreateTask("block-team", "Blocked task", "", "", []int{t1.ID}, "", "", "")

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
	CreateTask("prio-team", "Low priority", "", "", nil, PriorityLow, "", "")
	CreateTask("prio-team", "Normal priority", "", "", nil, PriorityNormal, "", "")
	CreateTask("prio-team", "High priority", "", "", nil, PriorityHigh, "", "")
	CreateTask("prio-team", "Default priority", "", "", nil, "", "", "")

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

	task, err := CreateTask("default-prio", "Test", "", "", nil, "", "", "")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.Priority != PriorityNormal {
		t.Errorf("Default priority = %s, want %s", task.Priority, PriorityNormal)
	}

	task2, err := CreateTask("default-prio", "High", "", "", nil, PriorityHigh, "", "")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task2.Priority != PriorityHigh {
		t.Errorf("Explicit priority = %s, want %s", task2.Priority, PriorityHigh)
	}
}

func TestRedirectTask(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("redirect-team", "", "")

	// Create and assign a task
	task, err := CreateTask("redirect-team", "Original task", "do original work", "worker1", nil, PriorityHigh, "myproject", "/tmp/work")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Transition to running
	UpdateTask("redirect-team", task.ID, func(t *Task) error {
		t.Status = TaskRunning
		return nil
	})

	// Redirect the task
	newTask, err := RedirectTask("redirect-team", task.ID, "new instructions for the work", "")
	if err != nil {
		t.Fatalf("RedirectTask: %v", err)
	}

	// Verify old task is cancelled
	oldTask, _ := GetTask("redirect-team", task.ID)
	if oldTask.Status != TaskCancelled {
		t.Errorf("Old task status = %s, want %s", oldTask.Status, TaskCancelled)
	}

	// Verify new task inherits properties
	if newTask.Owner != "worker1" {
		t.Errorf("New task owner = %q, want %q", newTask.Owner, "worker1")
	}
	if newTask.Priority != PriorityHigh {
		t.Errorf("New task priority = %s, want %s", newTask.Priority, PriorityHigh)
	}
	if newTask.Project != "myproject" {
		t.Errorf("New task project = %q, want %q", newTask.Project, "myproject")
	}
	if newTask.WorkDir != "/tmp/work" {
		t.Errorf("New task workdir = %q, want %q", newTask.WorkDir, "/tmp/work")
	}
	if newTask.Description != "new instructions for the work" {
		t.Errorf("New task description = %q, want %q", newTask.Description, "new instructions for the work")
	}
	// Subject should be inherited
	if newTask.Subject != "Original task" {
		t.Errorf("New task subject = %q, want %q (inherited)", newTask.Subject, "Original task")
	}
	// Status should be assigned (since owner is set)
	if newTask.Status != TaskAssigned {
		t.Errorf("New task status = %s, want %s", newTask.Status, TaskAssigned)
	}
}

func TestRedirectTaskWithNewSubject(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("redirect-subj", "", "")

	task, _ := CreateTask("redirect-subj", "Old subject", "old desc", "worker1", nil, "", "", "")
	UpdateTask("redirect-subj", task.ID, func(t *Task) error {
		t.Status = TaskRunning
		return nil
	})

	newTask, err := RedirectTask("redirect-subj", task.ID, "new desc", "New subject")
	if err != nil {
		t.Fatalf("RedirectTask: %v", err)
	}
	if newTask.Subject != "New subject" {
		t.Errorf("Subject = %q, want %q", newTask.Subject, "New subject")
	}
}

func TestRedirectTaskCannotRedirectCompleted(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("redirect-fail", "", "")

	task, _ := CreateTask("redirect-fail", "Done task", "", "worker1", nil, "", "", "")
	AssignTask("redirect-fail", task.ID, "worker1")
	CompleteTask("redirect-fail", task.ID, "all done")

	_, err := RedirectTask("redirect-fail", task.ID, "new work", "")
	if err == nil {
		t.Error("RedirectTask on completed task should fail")
	}
}

func TestCheckTaskCancellation(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("cancel-team", "", "")

	task, _ := CreateTask("cancel-team", "Cancel me", "", "worker1", nil, "", "", "")
	UpdateTask("cancel-team", task.ID, func(t *Task) error {
		t.Status = TaskRunning
		return nil
	})

	// Simulate daemon with a running task
	cancelled := false
	d := &Daemon{
		TeamName:    "cancel-team",
		AgentName:   "worker1",
		runningTask: task.ID,
		taskCancel:  func() { cancelled = true },
		logger:      newTestLogger(),
	}

	// Task is running — checkTaskCancellation should NOT cancel
	d.checkTaskCancellation()
	if cancelled {
		t.Error("Should not cancel a running task")
	}

	// Cancel the task externally
	CancelTask("cancel-team", task.ID)

	// Now checkTaskCancellation should trigger cancel
	d.checkTaskCancellation()
	if !cancelled {
		t.Error("Should have called taskCancel after task was cancelled externally")
	}
}

func TestHandleTaskResultCancelled(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("result-cancel", "", "")

	task, _ := CreateTask("result-cancel", "Cancelled task", "", "worker1", nil, "", "", "")
	UpdateTask("result-cancel", task.ID, func(t *Task) error {
		t.Status = TaskRunning
		return nil
	})

	// Cancel externally
	CancelTask("result-cancel", task.ID)

	d := &Daemon{
		TeamName:  "result-cancel",
		AgentName: "worker1",
		logger:    newTestLogger(),
	}

	state := &AgentState{
		Name:   "worker1",
		Team:   "result-cancel",
		Status: AgentRunning,
	}

	// Simulate task returning with a partial result after cancellation
	res := taskResult{
		task:   task,
		result: &ClaudeResult{Result: "partial work done"},
		err:    nil,
	}

	d.handleTaskResult(res, state)

	// Clean up notification file written by writeNotification to avoid
	// interfering with MCP monitor E2E tests that scan ~/.codes/notifications/.
	if home, err := os.UserHomeDir(); err == nil {
		os.Remove(filepath.Join(home, ".codes", "notifications", "result-cancel__1.json"))
	}

	// Verify partial result was saved
	updated, _ := GetTask("result-cancel", task.ID)
	if updated.Result == "" {
		t.Error("Expected partial result to be saved")
	}
	if updated.Status != TaskCancelled {
		t.Errorf("Status = %s, want %s", updated.Status, TaskCancelled)
	}

	// Verify state was reset to idle
	if state.Status != AgentIdle {
		t.Errorf("Agent status = %s, want %s", state.Status, AgentIdle)
	}
	if state.CurrentTask != 0 {
		t.Errorf("CurrentTask = %d, want 0", state.CurrentTask)
	}
}

func TestSendCallbackSuccess(t *testing.T) {
	var received taskNotification
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := &Daemon{
		TeamName:  "cb-team",
		AgentName: "worker",
		logger:    newTestLogger(),
	}

	n := taskNotification{
		Team:    "cb-team",
		TaskID:  42,
		Subject: "Do something",
		Status:  "completed",
		Agent:   "worker",
		Result:  "all done",
	}
	d.sendCallback(srv.URL, n)

	if received.TaskID != 42 {
		t.Errorf("callback taskId = %d, want 42", received.TaskID)
	}
	if received.Status != "completed" {
		t.Errorf("callback status = %s, want completed", received.Status)
	}
	if received.Result != "all done" {
		t.Errorf("callback result = %q, want 'all done'", received.Result)
	}
}

func TestSendCallbackNonOKStatus(t *testing.T) {
	// Non-2xx response should be logged but not panic or return error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := &Daemon{
		TeamName:  "cb-team",
		AgentName: "worker",
		logger:    newTestLogger(),
	}
	// Should not panic
	d.sendCallback(srv.URL, taskNotification{Team: "cb-team", TaskID: 1, Status: "failed"})
}

func TestSendCallbackUnreachable(t *testing.T) {
	d := &Daemon{
		TeamName:  "cb-team",
		AgentName: "worker",
		logger:    newTestLogger(),
	}
	// Unreachable URL — should not panic, just log
	d.sendCallback("http://127.0.0.1:1", taskNotification{Team: "cb-team", TaskID: 1, Status: "completed"})
}

func TestTaskCallbackURLPersisted(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	CreateTeam("cb-persist", "", "")

	task, err := CreateTask("cb-persist", "Test callback", "", "worker1", nil, "", "", "")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	updated, err := UpdateTask("cb-persist", task.ID, func(t *Task) error {
		t.CallbackURL = "https://example.com/callback"
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if updated.CallbackURL != "https://example.com/callback" {
		t.Errorf("CallbackURL = %q, want %q", updated.CallbackURL, "https://example.com/callback")
	}

	// Verify it survives a round-trip read
	loaded, err := GetTask("cb-persist", task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if loaded.CallbackURL != "https://example.com/callback" {
		t.Errorf("Loaded CallbackURL = %q, want %q", loaded.CallbackURL, "https://example.com/callback")
	}
}
