package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"codes/internal/agent"
	"codes/internal/output"
	"codes/internal/ui"
)

// -- Team commands --

func RunAgentTeamCreate(name, description, workdir string) {
	cfg, err := agent.CreateTeam(name, description, workdir)
	if err != nil {
		ui.ShowError("Failed to create team", err)
		return
	}

	if output.JSONMode {
		printJSON(cfg)
		return
	}
	ui.ShowSuccess("Team %q created", cfg.Name)
}

func RunAgentTeamDelete(name string) {
	if err := agent.DeleteTeam(name); err != nil {
		ui.ShowError("Failed to delete team", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]bool{"deleted": true})
		return
	}
	ui.ShowSuccess("Team %q deleted", name)
}

func RunAgentTeamList() {
	teams, err := agent.ListTeams()
	if err != nil {
		ui.ShowError("Failed to list teams", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{"teams": teams})
		return
	}

	if len(teams) == 0 {
		fmt.Println("No teams configured")
		return
	}

	for _, name := range teams {
		cfg, err := agent.GetTeam(name)
		if err != nil {
			fmt.Printf("  %s (error: %s)\n", name, err)
			continue
		}
		fmt.Printf("  %s — %d members", name, len(cfg.Members))
		if cfg.Description != "" {
			fmt.Printf(" — %s", cfg.Description)
		}
		fmt.Println()
	}
}

func RunAgentTeamInfo(name string) {
	cfg, err := agent.GetTeam(name)
	if err != nil {
		ui.ShowError("Failed to get team info", err)
		return
	}

	if output.JSONMode {
		printJSON(cfg)
		return
	}

	fmt.Printf("Team: %s\n", cfg.Name)
	if cfg.Description != "" {
		fmt.Printf("Description: %s\n", cfg.Description)
	}
	if cfg.WorkDir != "" {
		fmt.Printf("WorkDir: %s\n", cfg.WorkDir)
	}
	fmt.Printf("Created: %s\n", cfg.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Members (%d):\n", len(cfg.Members))
	for _, m := range cfg.Members {
		fmt.Printf("  - %s", m.Name)
		if m.Role != "" {
			fmt.Printf(" (%s)", m.Role)
		}
		if m.Model != "" {
			fmt.Printf(" [%s]", m.Model)
		}

		// Show live status
		state, _ := agent.GetAgentState(name, m.Name)
		if state != nil {
			fmt.Printf(" — %s", state.Status)
			if state.CurrentTask > 0 {
				fmt.Printf(" (task #%d)", state.CurrentTask)
			}
		}
		fmt.Println()
	}
}

// -- Agent member commands --

func RunAgentAdd(teamName, agentName, role, model, agentType string) {
	member := agent.TeamMember{
		Name:  agentName,
		Role:  role,
		Model: model,
		Type:  agentType,
	}

	if err := agent.AddMember(teamName, member); err != nil {
		ui.ShowError("Failed to add agent", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]bool{"added": true})
		return
	}
	ui.ShowSuccess("Agent %q added to team %q", agentName, teamName)
}

func RunAgentRemove(teamName, agentName string) {
	if err := agent.RemoveMember(teamName, agentName); err != nil {
		ui.ShowError("Failed to remove agent", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]bool{"removed": true})
		return
	}
	ui.ShowSuccess("Agent %q removed from team %q", agentName, teamName)
}

func RunAgentStart(teamName, agentName string) {
	// Find our own binary
	exe, err := os.Executable()
	if err != nil {
		ui.ShowError("Cannot find executable", err)
		return
	}

	cmd := exec.Command(exe, "agent", "run", teamName, agentName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		ui.ShowError("Failed to start agent", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{"started": true, "pid": cmd.Process.Pid})
		return
	}
	ui.ShowSuccess("Agent %q started (pid %d)", agentName, cmd.Process.Pid)

	// Detach
	cmd.Process.Release()
}

func RunAgentStop(teamName, agentName string) {
	// Send stop message
	_, err := agent.SendMessage(teamName, "__system__", agentName, "__stop__")
	if err != nil {
		ui.ShowError("Failed to send stop signal", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]bool{"stopping": true})
		return
	}
	ui.ShowSuccess("Stop signal sent to agent %q", agentName)
}

func RunAgentDaemon(teamName, agentName string) {
	d, err := agent.NewDaemon(teamName, agentName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := d.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
		os.Exit(1)
	}
}

// -- Task commands --

func RunAgentTaskCreate(teamName, subject, description, assign string, blockedBy []int, priority string) {
	task, err := agent.CreateTask(teamName, subject, description, assign, blockedBy, agent.TaskPriority(priority))
	if err != nil {
		ui.ShowError("Failed to create task", err)
		return
	}

	if output.JSONMode {
		printJSON(task)
		return
	}
	fmt.Printf("Task #%d created: %s\n", task.ID, task.Subject)
	if task.Owner != "" {
		fmt.Printf("  Assigned to: %s\n", task.Owner)
	}
}

func RunAgentTaskList(teamName, statusFilter, ownerFilter string) {
	tasks, err := agent.ListTasks(teamName, agent.TaskStatus(statusFilter), ownerFilter)
	if err != nil {
		ui.ShowError("Failed to list tasks", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{"tasks": tasks})
		return
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found")
		return
	}

	for _, t := range tasks {
		status := string(t.Status)
		fmt.Printf("  #%d [%s] %s", t.ID, status, t.Subject)
		if t.Owner != "" {
			fmt.Printf(" → %s", t.Owner)
		}
		fmt.Println()
	}
}

func RunAgentTaskGet(teamName, taskIDStr string) {
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		ui.ShowError("Invalid task ID", fmt.Errorf("%s is not a number", taskIDStr))
		return
	}

	task, err := agent.GetTask(teamName, taskID)
	if err != nil {
		ui.ShowError("Failed to get task", err)
		return
	}

	if output.JSONMode {
		printJSON(task)
		return
	}

	fmt.Printf("Task #%d: %s\n", task.ID, task.Subject)
	fmt.Printf("  Status: %s\n", task.Status)
	if task.Owner != "" {
		fmt.Printf("  Owner: %s\n", task.Owner)
	}
	if task.Description != "" {
		fmt.Printf("  Description: %s\n", task.Description)
	}
	if len(task.BlockedBy) > 0 {
		fmt.Printf("  Blocked by: %v\n", task.BlockedBy)
	}
	if task.SessionID != "" {
		fmt.Printf("  Session: %s\n", task.SessionID)
	}
	if task.Result != "" {
		fmt.Printf("  Result: %s\n", task.Result)
	}
	if task.Error != "" {
		fmt.Printf("  Error: %s\n", task.Error)
	}
	fmt.Printf("  Created: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	if task.CompletedAt != nil {
		fmt.Printf("  Completed: %s\n", task.CompletedAt.Format("2006-01-02 15:04:05"))
	}
}

func RunAgentTaskCancel(teamName, taskIDStr string) {
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		ui.ShowError("Invalid task ID", fmt.Errorf("%s is not a number", taskIDStr))
		return
	}

	task, err := agent.CancelTask(teamName, taskID)
	if err != nil {
		ui.ShowError("Failed to cancel task", err)
		return
	}

	if output.JSONMode {
		printJSON(task)
		return
	}
	ui.ShowSuccess("Task #%d cancelled", task.ID)
}

// -- Message commands --

func RunAgentMessageSend(teamName, from, to, content string) {
	msg, err := agent.SendMessage(teamName, from, to, content)
	if err != nil {
		ui.ShowError("Failed to send message", err)
		return
	}

	if output.JSONMode {
		printJSON(msg)
		return
	}
	target := to
	if target == "" {
		target = "broadcast"
	}
	ui.ShowSuccess("Message sent: %s → %s", from, target)
}

func RunAgentMessageList(teamName, agentName string) {
	msgs, err := agent.GetMessages(teamName, agentName, false)
	if err != nil {
		ui.ShowError("Failed to list messages", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{"messages": msgs})
		return
	}

	if len(msgs) == 0 {
		fmt.Println("No messages")
		return
	}

	for _, m := range msgs {
		readMark := " "
		if !m.Read {
			readMark = "*"
		}
		target := m.To
		if target == "" {
			target = "broadcast"
		}
		fmt.Printf("  %s [%s] %s → %s: %s\n",
			readMark,
			m.CreatedAt.Format("15:04:05"),
			m.From, target, m.Content)
	}
}

// -- Status command --

func RunAgentStatus(teamName string) {
	cfg, err := agent.GetTeam(teamName)
	if err != nil {
		ui.ShowError("Failed to get team status", err)
		return
	}

	tasks, _ := agent.ListTasks(teamName, "", "")

	if output.JSONMode {
		var agents []any
		for _, m := range cfg.Members {
			state, _ := agent.GetAgentState(teamName, m.Name)
			agents = append(agents, map[string]any{
				"member": m,
				"state":  state,
			})
		}
		printJSON(map[string]any{
			"team":   cfg,
			"agents": agents,
			"tasks":  tasks,
		})
		return
	}

	fmt.Printf("=== Team: %s ===\n", cfg.Name)

	// Agents
	fmt.Printf("\nAgents (%d):\n", len(cfg.Members))
	for _, m := range cfg.Members {
		state, _ := agent.GetAgentState(teamName, m.Name)
		status := "not started"
		if state != nil {
			status = string(state.Status)
			if state.CurrentTask > 0 {
				status += fmt.Sprintf(" (task #%d)", state.CurrentTask)
			}
		}
		fmt.Printf("  %-15s %s\n", m.Name, status)
	}

	// Task summary
	counts := map[agent.TaskStatus]int{}
	for _, t := range tasks {
		counts[t.Status]++
	}
	fmt.Printf("\nTasks (%d total):\n", len(tasks))
	for _, s := range []agent.TaskStatus{
		agent.TaskPending, agent.TaskAssigned, agent.TaskRunning,
		agent.TaskCompleted, agent.TaskFailed, agent.TaskCancelled,
	} {
		if c := counts[s]; c > 0 {
			fmt.Printf("  %-12s %d\n", s, c)
		}
	}
}

// RunAgentStatusWatch runs RunAgentStatus in a loop, refreshing every 3 seconds.
func RunAgentStatusWatch(teamName string) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Print initial status
	fmt.Print("\033[2J\033[H") // clear screen + move cursor to top
	RunAgentStatus(teamName)
	fmt.Println("\n[watching, refresh every 3s — Ctrl+C to stop]")

	for {
		select {
		case <-sigCh:
			fmt.Println("\nStopped watching.")
			return
		case <-ticker.C:
			fmt.Print("\033[2J\033[H")
			RunAgentStatus(teamName)
			fmt.Println("\n[watching, refresh every 3s — Ctrl+C to stop]")
		}
	}
}

// printJSON is a helper to output JSON.
func printJSON(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

// -- Start-all / Stop-all commands --

func RunAgentStartAll(teamName string) {
	cfg, err := agent.GetTeam(teamName)
	if err != nil {
		ui.ShowError("Failed to get team", err)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		ui.ShowError("Cannot find executable", err)
		return
	}

	type result struct {
		Name    string `json:"name"`
		Started bool   `json:"started"`
		PID     int    `json:"pid,omitempty"`
		Error   string `json:"error,omitempty"`
	}

	var results []result
	for _, m := range cfg.Members {
		r := result{Name: m.Name}

		if agent.IsAgentAlive(teamName, m.Name) {
			r.Error = "already running"
			if state, _ := agent.GetAgentState(teamName, m.Name); state != nil {
				r.PID = state.PID
			}
			results = append(results, r)
			if !output.JSONMode {
				fmt.Printf("  %-15s already running (pid %d)\n", m.Name, r.PID)
			}
			continue
		}

		cmd := exec.Command(exe, "agent", "run", teamName, m.Name)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			r.Error = err.Error()
			results = append(results, r)
			if !output.JSONMode {
				fmt.Printf("  %-15s failed: %v\n", m.Name, err)
			}
			continue
		}

		r.Started = true
		r.PID = cmd.Process.Pid
		cmd.Process.Release()
		results = append(results, r)

		if !output.JSONMode {
			fmt.Printf("  %-15s started (pid %d)\n", m.Name, r.PID)
		}
	}

	if output.JSONMode {
		printJSON(map[string]any{"results": results})
	}
}

func RunAgentStopAll(teamName string) {
	cfg, err := agent.GetTeam(teamName)
	if err != nil {
		ui.ShowError("Failed to get team", err)
		return
	}

	type result struct {
		Name     string `json:"name"`
		Stopping bool   `json:"stopping"`
		Error    string `json:"error,omitempty"`
	}

	var results []result
	for _, m := range cfg.Members {
		r := result{Name: m.Name}
		_, err := agent.SendMessage(teamName, "__system__", m.Name, "__stop__")
		if err != nil {
			r.Error = err.Error()
			if !output.JSONMode {
				fmt.Printf("  %-15s error: %v\n", m.Name, err)
			}
		} else {
			r.Stopping = true
			if !output.JSONMode {
				fmt.Printf("  %-15s stop signal sent\n", m.Name)
			}
		}
		results = append(results, r)
	}

	if output.JSONMode {
		printJSON(map[string]any{"results": results})
	}
}
