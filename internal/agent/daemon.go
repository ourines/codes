package agent

import (
	"codes/internal/config"
	"codes/internal/notify"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Daemon manages the poll loop for an agent, executing assigned tasks
// and responding to messages from the team lead or other agents.
type Daemon struct {
	TeamName  string
	AgentName string
	Role      string
	Model     string
	WorkDir   string

	pollInterval time.Duration
	logger       *log.Logger
	msgSessionID string // established message session ID (set after first response)
}

// NewDaemon creates a new agent daemon.
func NewDaemon(teamName, agentName string) (*Daemon, error) {
	cfg, err := GetTeam(teamName)
	if err != nil {
		return nil, err
	}

	var member *TeamMember
	for i := range cfg.Members {
		if cfg.Members[i].Name == agentName {
			member = &cfg.Members[i]
			break
		}
	}
	if member == nil {
		return nil, fmt.Errorf("agent %q not found in team %q", agentName, teamName)
	}

	workDir := cfg.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	return &Daemon{
		TeamName:     teamName,
		AgentName:    agentName,
		Role:         member.Role,
		Model:        member.Model,
		WorkDir:      workDir,
		pollInterval: 3 * time.Second,
		logger:       log.New(os.Stderr, fmt.Sprintf("[agent:%s] ", agentName), log.LstdFlags),
	}, nil
}

// buildSystemPrompt generates a system prompt that gives the Claude subprocess
// awareness of its role, team context, and working conventions.
func (d *Daemon) buildSystemPrompt() string {
	return d.buildSystemPromptWithContext("", d.WorkDir)
}

// buildSystemPromptWithContext generates a system prompt with optional project context.
// When projectName is non-empty, additional project context is included in the prompt.
func (d *Daemon) buildSystemPromptWithContext(projectName, workDir string) string {
	role := d.Role
	if role == "" {
		role = "general-purpose worker"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb,
		"You are %q, a team member in team %q.\n"+
			"Your role: %s\n"+
			"Working directory: %s\n",
		d.AgentName, d.TeamName, role, workDir,
	)

	if projectName != "" {
		sb.WriteString(fmt.Sprintf("Project: %s\n", projectName))
	}

	sb.WriteString("\nInstructions:\n")
	sb.WriteString("- Complete your assigned tasks thoroughly and report results clearly.\n")
	sb.WriteString("- If a task is unclear, do your best interpretation and note any assumptions.\n")
	sb.WriteString("- Focus on the task at hand. Be concise in responses.")

	return sb.String()
}

// Run starts the daemon poll loop. It blocks until the context is cancelled
// or a stop message is received.
//
// The loop has three responsibilities each tick:
//   1. Check for __stop__ signal
//   2. Process incoming chat messages (respond via Claude, reply to sender)
//   3. Pick up and execute the next assigned task
func (d *Daemon) Run(ctx context.Context) error {
	// Record agent state with a persistent session ID for message conversations
	state := &AgentState{
		Name:      d.AgentName,
		Team:      d.TeamName,
		PID:       os.Getpid(),
		Status:    AgentIdle,
		SessionID: generateID(),
		StartedAt: time.Now(),
	}
	if err := SaveAgentState(state); err != nil {
		return fmt.Errorf("save agent state: %w", err)
	}

	d.logger.Printf("started (pid=%d, team=%s, session=%s)", state.PID, d.TeamName, state.SessionID)

	// Announce availability to the team
	BroadcastMessage(d.TeamName, d.AgentName, fmt.Sprintf("Agent %s is online and ready for tasks.", d.AgentName))

	defer func() {
		state.Status = AgentStopped
		SaveAgentState(state)
		BroadcastMessage(d.TeamName, d.AgentName, fmt.Sprintf("Agent %s is going offline.", d.AgentName))
		d.logger.Println("stopped")
	}()

	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// 1. Check for stop signal
			if d.shouldStop() {
				d.logger.Println("received stop signal")
				return nil
			}

			// 2. Process incoming chat messages
			d.processMessages(ctx, state)

			// 3. Look for an assigned task to execute
			task, err := d.findNextTask()
			if err != nil {
				d.logger.Printf("error finding task: %v", err)
				continue
			}
			if task == nil {
				continue
			}

			// Execute the task
			state.Status = AgentRunning
			state.CurrentTask = task.ID
			SaveAgentState(state)

			d.executeTask(ctx, task, state)

			state.Status = AgentIdle
			state.CurrentTask = 0
			SaveAgentState(state)
		}
	}
}

// shouldStop checks if there's a stop message for this agent.
func (d *Daemon) shouldStop() bool {
	msgs, err := GetMessages(d.TeamName, d.AgentName, true)
	if err != nil {
		return false
	}
	for _, msg := range msgs {
		if msg.Content == "__stop__" {
			MarkRead(d.TeamName, msg.ID)
			return true
		}
	}
	return false
}

// processMessages handles incoming chat messages by feeding them to Claude
// and sending the response back to the sender.
func (d *Daemon) processMessages(ctx context.Context, state *AgentState) {
	msgs, err := GetMessages(d.TeamName, d.AgentName, true)
	if err != nil {
		return
	}

	for _, msg := range msgs {
		// Skip system messages (already handled by shouldStop)
		if msg.Content == "__stop__" {
			continue
		}
		// Skip messages from self (prevents broadcast echo loops)
		if msg.From == d.AgentName {
			MarkRead(d.TeamName, msg.ID)
			continue
		}
		// Skip auto-reports (don't respond to task_completed/task_failed notifications)
		if msg.Type == MsgTaskCompleted || msg.Type == MsgTaskFailed || msg.Type == MsgSystem {
			MarkRead(d.TeamName, msg.ID)
			continue
		}
		// Skip broadcast messages — only respond to direct messages
		// Broadcasts are informational (e.g. "agent online"); responding creates message storms.
		if msg.To == "" {
			d.logger.Printf("broadcast from %s: %s (read-only)", msg.From, truncate(msg.Content, 80))
			MarkRead(d.TeamName, msg.ID)
			continue
		}

		d.logger.Printf("message from %s: %s", msg.From, truncate(msg.Content, 80))
		MarkRead(d.TeamName, msg.ID)

		// Build a prompt that includes the message context
		prompt := fmt.Sprintf(
			"You received a message from %q:\n\n%s\n\nRespond concisely and helpfully. If this is a work request, do the work and report results.",
			msg.From, msg.Content,
		)

		opts := RunOptions{
			Prompt:       prompt,
			WorkDir:      d.WorkDir,
			Model:        d.Model,
			SystemPrompt: d.buildSystemPrompt(),
			PermMode:     "dangerously-skip-permissions",
		}
		// Resume existing message session if one was established
		if d.msgSessionID != "" {
			opts.SessionID = d.msgSessionID
			opts.Resume = true
		}

		// Use default adapter (claude) for message handling
		result, err := RunClaude(ctx, opts)
		if err != nil {
			d.logger.Printf("error responding to message: %v", err)
			SendMessage(d.TeamName, d.AgentName, msg.From,
				fmt.Sprintf("[error] Failed to process your message: %v", err))
			continue
		}

		// Track the session ID for future message continuity
		if result.SessionID != "" {
			d.msgSessionID = result.SessionID
		}

		// Send the response back to the sender
		response := result.Result
		if result.IsError {
			response = fmt.Sprintf("[error] %s", result.Error)
		}
		if response == "" {
			response = "(no response generated)"
		}

		SendMessage(d.TeamName, d.AgentName, msg.From, response)
		d.logger.Printf("replied to %s", msg.From)
	}
}

// findNextTask finds the next task for this agent. It first looks for tasks
// explicitly assigned to this agent, then auto-claims unassigned pending tasks.
func (d *Daemon) findNextTask() (*Task, error) {
	// 1. Check for tasks explicitly assigned to this agent
	tasks, err := ListTasks(d.TeamName, TaskAssigned, d.AgentName)
	if err != nil {
		return nil, err
	}

	for _, task := range tasks {
		blocked, err := IsTaskBlocked(d.TeamName, task)
		if err != nil {
			continue
		}
		if !blocked {
			return task, nil
		}
	}

	// 2. Auto-claim unassigned pending tasks
	pending, err := ListTasks(d.TeamName, TaskPending, "")
	if err != nil {
		return nil, err
	}

	for _, task := range pending {
		if task.Owner != "" {
			continue
		}
		blocked, err := IsTaskBlocked(d.TeamName, task)
		if err != nil {
			continue
		}
		if blocked {
			continue
		}

		// Claim the task
		claimed, err := UpdateTask(d.TeamName, task.ID, func(t *Task) error {
			// Double-check it's still unclaimed
			if t.Owner != "" || t.Status != TaskPending {
				return fmt.Errorf("task already claimed")
			}
			t.Owner = d.AgentName
			t.Status = TaskAssigned
			return nil
		})
		if err != nil {
			continue // another agent may have claimed it
		}

		d.logger.Printf("auto-claimed task %d: %s", claimed.ID, claimed.Subject)
		return claimed, nil
	}

	return nil, nil
}

// executeTask runs a Claude subprocess for the given task and auto-reports
// the result back to the team lead via messages.
func (d *Daemon) executeTask(ctx context.Context, task *Task, state *AgentState) {
	d.logger.Printf("executing task %d: %s", task.ID, task.Subject)

	// Transition to running
	_, err := UpdateTask(d.TeamName, task.ID, func(t *Task) error {
		t.Status = TaskRunning
		return nil
	})
	if err != nil {
		d.logger.Printf("error updating task %d to running: %v", task.ID, err)
		return
	}

	// Build prompt
	prompt := task.Subject
	if task.Description != "" {
		prompt = fmt.Sprintf("%s\n\n%s", task.Subject, task.Description)
	}

	// Resolve task-specific working directory:
	//   1. Explicit task.WorkDir takes highest precedence
	//   2. task.Project resolves via config.GetProjectPath()
	//   3. Fall back to daemon's default WorkDir
	taskWorkDir := d.WorkDir
	taskProject := ""
	if task.WorkDir != "" {
		taskWorkDir = task.WorkDir
	} else if task.Project != "" {
		if projectPath, ok := config.GetProjectPath(task.Project); ok {
			taskWorkDir = projectPath
			taskProject = task.Project
			d.logger.Printf("task %d: project %q → %s", task.ID, task.Project, projectPath)
		} else {
			d.logger.Printf("warning: project %q not found in config, using default workdir", task.Project)
		}
	}

	opts := RunOptions{
		Prompt:       prompt,
		WorkDir:      taskWorkDir,
		Model:        d.Model,
		SystemPrompt: d.buildSystemPromptWithContext(taskProject, taskWorkDir),
		PermMode:     "dangerously-skip-permissions",
	}
	// Resume existing task session if available (for retries/continuations)
	if task.SessionID != "" {
		opts.SessionID = task.SessionID
		opts.Resume = true
	}

	// Determine which adapter to use
	adapterName := task.Adapter
	if adapterName == "" {
		adapterName = "claude" // Default to claude
	}

	var result *ClaudeResult
	result, err = RunWithAdapter(ctx, adapterName, opts)

	if err != nil {
		d.logger.Printf("error executing task %d with adapter %s: %v", task.ID, adapterName, err)
		FailTask(d.TeamName, task.ID, err.Error())
		d.reportTaskFailed(task, err.Error())
		return
	}

	if result.IsError {
		d.logger.Printf("task %d failed: %s", task.ID, result.Error)
		FailTask(d.TeamName, task.ID, result.Error)
		d.reportTaskFailed(task, result.Error)
		return
	}

	d.logger.Printf("task %d completed", task.ID)

	// Update session ID from result if available
	if result.SessionID != "" {
		UpdateTask(d.TeamName, task.ID, func(t *Task) error {
			t.SessionID = result.SessionID
			return nil
		})
	}

	CompleteTask(d.TeamName, task.ID, result.Result)
	d.reportTaskCompleted(task, result.Result)
}

// reportTaskCompleted broadcasts a task completion report to the team.
func (d *Daemon) reportTaskCompleted(task *Task, result string) {
	summary := truncate(result, 500)
	content := fmt.Sprintf("Task #%d completed: %s\n\nResult: %s", task.ID, task.Subject, summary)

	// Send to all (broadcast) so leader and other agents can see
	SendTaskReport(d.TeamName, d.AgentName, "", MsgTaskCompleted, task.ID, content)

	// Write notification file for external consumers
	d.writeNotification(task, "completed", result)
}

// reportTaskFailed broadcasts a task failure report to the team.
func (d *Daemon) reportTaskFailed(task *Task, errMsg string) {
	content := fmt.Sprintf("Task #%d FAILED: %s\n\nError: %s", task.ID, task.Subject, errMsg)
	SendTaskReport(d.TeamName, d.AgentName, "", MsgTaskFailed, task.ID, content)

	// Write notification file for external consumers
	d.writeNotification(task, "failed", errMsg)
}

// taskNotification is the JSON structure written to ~/.codes/notifications/.
type taskNotification struct {
	Team      string `json:"team"`
	TaskID    int    `json:"taskId"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	Agent     string `json:"agent"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}

// writeNotification writes a notification file for a completed or failed task.
func (d *Daemon) writeNotification(task *Task, status, detail string) {
	home, err := os.UserHomeDir()
	if err != nil {
		d.logger.Printf("notification: cannot get home dir: %v", err)
		return
	}

	dir := filepath.Join(home, ".codes", "notifications")
	if err := os.MkdirAll(dir, 0755); err != nil {
		d.logger.Printf("notification: cannot create dir: %v", err)
		return
	}

	n := taskNotification{
		Team:      d.TeamName,
		TaskID:    task.ID,
		Subject:   task.Subject,
		Status:    status,
		Agent:     d.AgentName,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if status == "completed" {
		n.Result = truncate(detail, 500)
	} else {
		n.Error = detail
	}

	data, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		d.logger.Printf("notification: marshal error: %v", err)
		return
	}

	// Use __ separator to avoid ambiguity when team name contains hyphens
	filename := filepath.Join(dir, fmt.Sprintf("%s__%d.json", d.TeamName, task.ID))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		d.logger.Printf("notification: write error: %v", err)
	}

	// Send desktop notification
	notifier := notify.NewDesktopNotifier()
	if err := notifier.Send(notify.Notification{
		Title:   fmt.Sprintf("codes: Task %s", status),
		Message: fmt.Sprintf("[%s] #%d %s", d.TeamName, task.ID, task.Subject),
		Sound:   status == "completed",
	}); err != nil {
		d.logger.Printf("notification: desktop notify error: %v", err)
	}

	// Send webhook notifications (if configured)
	d.sendWebhookNotifications(status, task)

	// Execute shell hook (if configured)
	d.executeHook(status, task, detail)
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// sendWebhookNotifications sends notifications to all configured webhooks.
func (d *Daemon) sendWebhookNotifications(status string, task *Task) {
	webhooks, err := config.ListWebhooks()
	if err != nil || len(webhooks) == 0 {
		return
	}

	// Determine event type
	eventType := "task_completed"
	if status == "failed" {
		eventType = "task_failed"
	}

	notification := notify.Notification{
		Title:   fmt.Sprintf("codes: Task %s", status),
		Message: fmt.Sprintf("[%s] #%d %s", d.TeamName, task.ID, task.Subject),
		Sound:   false, // webhooks don't use sound
	}

	for _, webhook := range webhooks {
		// Check event filter
		if len(webhook.Events) > 0 {
			allowed := false
			for _, event := range webhook.Events {
				if event == eventType {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		// Send notification
		notifier := notify.NewWebhookNotifier(webhook.URL, webhook.Format, webhook.Extra)
		if err := notifier.Send(notification); err != nil {
			d.logger.Printf("webhook notification error (%s): %v", webhook.URL, err)
		}
	}
}

// executeHook runs the shell hook script for the given task status.
func (d *Daemon) executeHook(status string, task *Task, detail string) {
	// Map status to event name
	event := "on_task_failed"
	if status == "completed" {
		event = "on_task_completed"
	}

	scriptPath := config.GetHook(event)
	if scriptPath == "" {
		return
	}

	payload := notify.HookPayload{
		Team:      d.TeamName,
		TaskID:    task.ID,
		Subject:   task.Subject,
		Status:    status,
		Agent:     d.AgentName,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if status == "completed" {
		payload.Result = truncate(detail, 500)
	} else {
		payload.Error = detail
	}

	runner := notify.NewHookRunner(scriptPath)
	if err := runner.Execute(payload); err != nil {
		d.logger.Printf("hook execution error (%s): %v", event, err)
	}
}
