package agent

import (
	"context"
	"fmt"
	"log"
	"os"
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
	role := d.Role
	if role == "" {
		role = "general-purpose worker"
	}
	return fmt.Sprintf(
		"You are %q, a team member in team %q.\n"+
			"Your role: %s\n"+
			"Working directory: %s\n\n"+
			"Instructions:\n"+
			"- Complete your assigned tasks thoroughly and report results clearly.\n"+
			"- If a task is unclear, do your best interpretation and note any assumptions.\n"+
			"- Focus on the task at hand. Be concise in responses.",
		d.AgentName, d.TeamName, role, d.WorkDir,
	)
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
		// Skip auto-reports (don't respond to task_completed/task_failed notifications)
		if msg.Type == MsgTaskCompleted || msg.Type == MsgTaskFailed || msg.Type == MsgSystem {
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
			SessionID:    state.SessionID,
			Resume:       true,
			Model:        d.Model,
			SystemPrompt: d.buildSystemPrompt(),
			PermMode:     "dangerously-skip-permissions",
		}

		result, err := RunClaude(ctx, opts)
		if err != nil {
			d.logger.Printf("error responding to message: %v", err)
			SendMessage(d.TeamName, d.AgentName, msg.From,
				fmt.Sprintf("[error] Failed to process your message: %v", err))
			continue
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
		if t.SessionID == "" {
			t.SessionID = generateID()
		}
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

	// Reload task to get sessionID
	task, _ = GetTask(d.TeamName, task.ID)

	opts := RunOptions{
		Prompt:       prompt,
		WorkDir:      d.WorkDir,
		SessionID:    task.SessionID,
		Model:        d.Model,
		SystemPrompt: d.buildSystemPrompt(),
		PermMode:     "dangerously-skip-permissions",
	}

	result, err := RunClaude(ctx, opts)
	if err != nil {
		d.logger.Printf("claude error for task %d: %v", task.ID, err)
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
}

// reportTaskFailed broadcasts a task failure report to the team.
func (d *Daemon) reportTaskFailed(task *Task, errMsg string) {
	content := fmt.Sprintf("Task #%d FAILED: %s\n\nError: %s", task.ID, task.Subject, errMsg)
	SendTaskReport(d.TeamName, d.AgentName, "", MsgTaskFailed, task.ID, content)
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
