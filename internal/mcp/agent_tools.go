package mcpserver

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"codes/internal/agent"
)

// mcpServer holds the server reference for the background notification monitor.
var mcpServer *mcpsdk.Server

// -- team_create --

type teamCreateInput struct {
	Name        string `json:"name" jsonschema:"Team name"`
	Description string `json:"description,omitempty" jsonschema:"Team description"`
	WorkDir     string `json:"workDir,omitempty" jsonschema:"Working directory for agents"`
}

type teamCreateOutput struct {
	Created bool              `json:"created"`
	Team    *agent.TeamConfig `json:"team"`
}

func teamCreateHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamCreateInput) (*mcpsdk.CallToolResult, teamCreateOutput, error) {
	if input.Name == "" {
		return nil, teamCreateOutput{}, fmt.Errorf("name is required")
	}
	cfg, err := agent.CreateTeam(input.Name, input.Description, input.WorkDir)
	if err != nil {
		return nil, teamCreateOutput{}, err
	}
	return nil, teamCreateOutput{Created: true, Team: cfg}, nil
}

// -- team_delete --

type teamDeleteInput struct {
	Name string `json:"name" jsonschema:"Team name to delete"`
}

type teamDeleteOutput struct {
	Deleted bool `json:"deleted"`
}

func teamDeleteHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamDeleteInput) (*mcpsdk.CallToolResult, teamDeleteOutput, error) {
	if err := agent.DeleteTeam(input.Name); err != nil {
		return nil, teamDeleteOutput{}, err
	}
	return nil, teamDeleteOutput{Deleted: true}, nil
}

// -- team_list --

type teamListInput struct{}

type teamListOutput struct {
	Teams []string `json:"teams"`
}

func teamListHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamListInput) (*mcpsdk.CallToolResult, teamListOutput, error) {
	teams, err := agent.ListTeams()
	if err != nil {
		return nil, teamListOutput{}, err
	}
	if teams == nil {
		teams = []string{}
	}
	return nil, teamListOutput{Teams: teams}, nil
}

// -- team_get --

type teamGetInput struct {
	Name string `json:"name" jsonschema:"Team name"`
}

type teamGetOutput struct {
	Team   *agent.TeamConfig `json:"team"`
	Agents []agentInfo       `json:"agents"`
}

type agentInfo struct {
	agent.TeamMember
	State *agent.AgentState `json:"state,omitempty"`
	Alive bool              `json:"alive"`
}

func teamGetHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamGetInput) (*mcpsdk.CallToolResult, teamGetOutput, error) {
	cfg, err := agent.GetTeam(input.Name)
	if err != nil {
		return nil, teamGetOutput{}, err
	}

	agents := make([]agentInfo, 0, len(cfg.Members))
	for _, m := range cfg.Members {
		info := agentInfo{TeamMember: m}
		state, _ := agent.GetAgentState(input.Name, m.Name)
		info.State = state
		info.Alive = agent.IsAgentAlive(input.Name, m.Name)
		agents = append(agents, info)
	}

	return nil, teamGetOutput{Team: cfg, Agents: agents}, nil
}

// -- agent_add --

type agentAddInput struct {
	Team  string `json:"team" jsonschema:"Team name"`
	Name  string `json:"name" jsonschema:"Agent name"`
	Role  string `json:"role,omitempty" jsonschema:"Agent role description"`
	Model string `json:"model,omitempty" jsonschema:"Claude model (e.g. sonnet, opus)"`
	Type  string `json:"type,omitempty" jsonschema:"Agent type (worker, leader)"`
}

type agentAddOutput struct {
	Added bool `json:"added"`
}

func agentAddHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input agentAddInput) (*mcpsdk.CallToolResult, agentAddOutput, error) {
	if input.Team == "" || input.Name == "" {
		return nil, agentAddOutput{}, fmt.Errorf("team and name are required")
	}
	member := agent.TeamMember{
		Name:  input.Name,
		Role:  input.Role,
		Model: input.Model,
		Type:  input.Type,
	}
	if err := agent.AddMember(input.Team, member); err != nil {
		return nil, agentAddOutput{}, err
	}
	return nil, agentAddOutput{Added: true}, nil
}

// -- agent_remove --

type agentRemoveInput struct {
	Team string `json:"team" jsonschema:"Team name"`
	Name string `json:"name" jsonschema:"Agent name to remove"`
}

type agentRemoveOutput struct {
	Removed bool `json:"removed"`
}

func agentRemoveHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input agentRemoveInput) (*mcpsdk.CallToolResult, agentRemoveOutput, error) {
	if err := agent.RemoveMember(input.Team, input.Name); err != nil {
		return nil, agentRemoveOutput{}, err
	}
	return nil, agentRemoveOutput{Removed: true}, nil
}

// -- agent_list --

type agentListInput struct {
	Team string `json:"team" jsonschema:"Team name"`
}

type agentListOutput struct {
	Agents        []agentInfo        `json:"agents"`
	Notifications []taskNotification `json:"pending_notifications,omitempty"`
}

func agentListHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input agentListInput) (*mcpsdk.CallToolResult, agentListOutput, error) {
	cfg, err := agent.GetTeam(input.Team)
	if err != nil {
		return nil, agentListOutput{}, err
	}

	agents := make([]agentInfo, 0, len(cfg.Members))
	for _, m := range cfg.Members {
		info := agentInfo{TeamMember: m}
		state, _ := agent.GetAgentState(input.Team, m.Name)
		info.State = state
		info.Alive = agent.IsAgentAlive(input.Team, m.Name)
		agents = append(agents, info)
	}

	return nil, agentListOutput{Agents: agents, Notifications: drainPendingNotifications()}, nil
}

// -- agent_start --

type agentStartInput struct {
	Team string `json:"team" jsonschema:"Team name"`
	Name string `json:"name" jsonschema:"Agent name to start"`
}

type agentStartOutput struct {
	Started       bool               `json:"started"`
	PID           int                `json:"pid,omitempty"`
	MonitorActive bool               `json:"monitor_active"`
	Notifications []taskNotification `json:"pending_notifications,omitempty"`
}

func agentStartHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input agentStartInput) (*mcpsdk.CallToolResult, agentStartOutput, error) {
	pid, err := agent.StartAgent(input.Team, input.Name)
	if err != nil {
		return nil, agentStartOutput{}, err
	}

	// Ensure background notification monitor is running
	ensureMonitorRunning(mcpServer)

	return nil, agentStartOutput{
		Started:       true,
		PID:           pid,
		MonitorActive: monitorRunning.Load(),
		Notifications: drainPendingNotifications(),
	}, nil
}

// -- agent_stop --

type agentStopInput struct {
	Team string `json:"team" jsonschema:"Team name"`
	Name string `json:"name" jsonschema:"Agent name to stop"`
}

type agentStopOutput struct {
	Stopping bool `json:"stopping"`
}

func agentStopHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input agentStopInput) (*mcpsdk.CallToolResult, agentStopOutput, error) {
	_, err := agent.SendMessage(input.Team, "__system__", input.Name, "__stop__")
	if err != nil {
		return nil, agentStopOutput{}, err
	}
	return nil, agentStopOutput{Stopping: true}, nil
}

// -- task_create --

type taskCreateInput struct {
	Team        string `json:"team" jsonschema:"Team name"`
	Subject     string `json:"subject" jsonschema:"Task subject/title"`
	Description string `json:"description,omitempty" jsonschema:"Detailed task description"`
	Assign      string `json:"assign,omitempty" jsonschema:"Agent name to assign the task to"`
	BlockedBy   []int  `json:"blockedBy,omitempty" jsonschema:"Task IDs that must complete before this task"`
	Priority    string `json:"priority,omitempty" jsonschema:"Task priority: high, normal, or low (default: normal)"`
	Project     string `json:"project,omitempty" jsonschema:"Project name to execute in (registered via add_project)"`
	WorkDir     string `json:"workDir,omitempty" jsonschema:"Explicit working directory (overrides project)"`
}

type taskCreateOutput struct {
	Task          *agent.Task        `json:"task"`
	MonitorActive bool               `json:"monitor_active"`
	Notifications []taskNotification `json:"pending_notifications,omitempty"`
}

func taskCreateHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input taskCreateInput) (*mcpsdk.CallToolResult, taskCreateOutput, error) {
	if input.Team == "" || input.Subject == "" {
		return nil, taskCreateOutput{}, fmt.Errorf("team and subject are required")
	}
	task, err := agent.CreateTask(input.Team, input.Subject, input.Description, input.Assign, input.BlockedBy, agent.TaskPriority(input.Priority), input.Project, input.WorkDir)
	if err != nil {
		return nil, taskCreateOutput{}, err
	}

	// Ensure background notification monitor is running
	ensureMonitorRunning(mcpServer)

	return nil, taskCreateOutput{
		Task:          task,
		MonitorActive: monitorRunning.Load(),
		Notifications: drainPendingNotifications(),
	}, nil
}

// -- task_update --

type taskUpdateInput struct {
	Team        string `json:"team" jsonschema:"Team name"`
	TaskID      int    `json:"taskId" jsonschema:"Task ID"`
	Status      string `json:"status,omitempty" jsonschema:"New status (pending, assigned, running, completed, failed, cancelled)"`
	Owner       string `json:"owner,omitempty" jsonschema:"New owner agent name"`
	Result      string `json:"result,omitempty" jsonschema:"Task result (for completing)"`
	Error       string `json:"error,omitempty" jsonschema:"Error message (for failing)"`
	Description string `json:"description,omitempty" jsonschema:"Updated description"`
}

type taskUpdateOutput struct {
	Task *agent.Task `json:"task"`
}

func taskUpdateHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input taskUpdateInput) (*mcpsdk.CallToolResult, taskUpdateOutput, error) {
	task, err := agent.UpdateTask(input.Team, input.TaskID, func(t *agent.Task) error {
		if input.Status != "" {
			t.Status = agent.TaskStatus(input.Status)
			// Auto-set StartedAt when transitioning to running
			if input.Status == "running" && t.StartedAt == nil {
				now := time.Now()
				t.StartedAt = &now
			}
		}
		if input.Owner != "" {
			t.Owner = input.Owner
		}
		if input.Result != "" {
			t.Result = input.Result
		}
		if input.Error != "" {
			t.Error = input.Error
		}
		if input.Description != "" {
			t.Description = input.Description
		}
		return nil
	})
	if err != nil {
		return nil, taskUpdateOutput{}, err
	}
	return nil, taskUpdateOutput{Task: task}, nil
}

// -- task_redirect --

type taskRedirectInput struct {
	Team            string `json:"team" jsonschema:"Team name"`
	TaskID          int    `json:"taskId" jsonschema:"Task ID of the running task to cancel and redirect"`
	NewInstructions string `json:"newInstructions" jsonschema:"New task description/instructions for the replacement task"`
	Subject         string `json:"subject,omitempty" jsonschema:"Optional new subject (inherits from original task if not provided)"`
}

type taskRedirectOutput struct {
	CancelledTaskID int        `json:"cancelled_task_id"`
	NewTask         *agent.Task `json:"new_task"`
}

func taskRedirectHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input taskRedirectInput) (*mcpsdk.CallToolResult, taskRedirectOutput, error) {
	if input.Team == "" || input.TaskID == 0 || input.NewInstructions == "" {
		return nil, taskRedirectOutput{}, fmt.Errorf("team, taskId, and newInstructions are required")
	}
	newTask, err := agent.RedirectTask(input.Team, input.TaskID, input.NewInstructions, input.Subject)
	if err != nil {
		return nil, taskRedirectOutput{}, err
	}
	return nil, taskRedirectOutput{
		CancelledTaskID: input.TaskID,
		NewTask:         newTask,
	}, nil
}

// -- task_list --

type taskListInput struct {
	Team   string `json:"team" jsonschema:"Team name"`
	Status string `json:"status,omitempty" jsonschema:"Filter by status"`
	Owner  string `json:"owner,omitempty" jsonschema:"Filter by owner"`
}

type taskListOutput struct {
	Tasks         []*agent.Task      `json:"tasks"`
	Notifications []taskNotification `json:"pending_notifications,omitempty"`
}

func taskListHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input taskListInput) (*mcpsdk.CallToolResult, taskListOutput, error) {
	tasks, err := agent.ListTasks(input.Team, agent.TaskStatus(input.Status), input.Owner)
	if err != nil {
		return nil, taskListOutput{}, err
	}
	if tasks == nil {
		tasks = []*agent.Task{}
	}
	return nil, taskListOutput{Tasks: tasks, Notifications: drainPendingNotifications()}, nil
}

// -- task_get --

type taskGetInput struct {
	Team   string `json:"team" jsonschema:"Team name"`
	TaskID int    `json:"taskId" jsonschema:"Task ID"`
}

type taskGetOutput struct {
	Task            *agent.Task        `json:"task"`
	RunningDuration string             `json:"runningDuration,omitempty"`
	Notifications   []taskNotification `json:"pending_notifications,omitempty"`
}

func taskGetHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input taskGetInput) (*mcpsdk.CallToolResult, taskGetOutput, error) {
	task, err := agent.GetTask(input.Team, input.TaskID)
	if err != nil {
		return nil, taskGetOutput{}, err
	}
	out := taskGetOutput{Task: task, Notifications: drainPendingNotifications()}
	if task.Status == agent.TaskRunning && task.StartedAt != nil {
		out.RunningDuration = time.Since(*task.StartedAt).Truncate(time.Second).String()
	}
	return nil, out, nil
}

// -- message_send --

type messageSendInput struct {
	Team    string `json:"team" jsonschema:"Team name"`
	From    string `json:"from" jsonschema:"Sender agent name"`
	To      string `json:"to,omitempty" jsonschema:"Recipient agent name (empty for broadcast)"`
	Content string `json:"content" jsonschema:"Message content"`
	Type    string `json:"type,omitempty" jsonschema:"Message type: chat|progress|help_request|discovery (default: chat)"`
	TaskID  int    `json:"taskId,omitempty" jsonschema:"Related task ID"`
}

type messageSendOutput struct {
	Message *agent.Message `json:"message"`
}

func messageSendHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input messageSendInput) (*mcpsdk.CallToolResult, messageSendOutput, error) {
	if input.Team == "" || input.From == "" || input.Content == "" {
		return nil, messageSendOutput{}, fmt.Errorf("team, from, and content are required")
	}
	msgType := agent.MsgChat
	if input.Type != "" {
		msgType = agent.MessageType(input.Type)
	}
	msg, err := agent.SendTypedMessage(input.Team, msgType, input.From, input.To, input.Content, input.TaskID)
	if err != nil {
		return nil, messageSendOutput{}, err
	}
	return nil, messageSendOutput{Message: msg}, nil
}

// -- message_list --

type messageListInput struct {
	Team       string `json:"team" jsonschema:"Team name"`
	Agent      string `json:"agent" jsonschema:"Agent name to list messages for"`
	UnreadOnly bool   `json:"unreadOnly,omitempty" jsonschema:"Only return unread messages"`
	Type       string `json:"type,omitempty" jsonschema:"Filter by message type (chat, task_completed, task_failed, system)"`
}

type messageListOutput struct {
	Messages []*agent.Message `json:"messages"`
}

func messageListHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input messageListInput) (*mcpsdk.CallToolResult, messageListOutput, error) {
	if input.Team == "" || input.Agent == "" {
		return nil, messageListOutput{}, fmt.Errorf("team and agent are required")
	}

	var msgs []*agent.Message
	var err error

	if input.Type != "" {
		msgs, err = agent.GetMessagesByType(input.Team, input.Agent, agent.MessageType(input.Type), input.UnreadOnly)
	} else {
		msgs, err = agent.GetMessages(input.Team, input.Agent, input.UnreadOnly)
	}
	if err != nil {
		return nil, messageListOutput{}, err
	}
	if msgs == nil {
		msgs = []*agent.Message{}
	}
	return nil, messageListOutput{Messages: msgs}, nil
}

// -- message_mark_read --

type messageMarkReadInput struct {
	Team      string `json:"team" jsonschema:"Team name"`
	MessageID string `json:"messageId" jsonschema:"Message ID to mark as read"`
}

type messageMarkReadOutput struct {
	MarkedRead bool `json:"markedRead"`
}

func messageMarkReadHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input messageMarkReadInput) (*mcpsdk.CallToolResult, messageMarkReadOutput, error) {
	if err := agent.MarkRead(input.Team, input.MessageID); err != nil {
		return nil, messageMarkReadOutput{}, err
	}
	return nil, messageMarkReadOutput{MarkedRead: true}, nil
}

// -- team_status --

type teamStatusInput struct {
	Name string `json:"name" jsonschema:"Team name"`
}

type teamStatusAgentInfo struct {
	Name               string `json:"name"`
	Status             string `json:"status"`
	Alive              bool   `json:"alive"`
	CurrentTask        int    `json:"currentTask,omitempty"`
	CurrentTaskSubject string `json:"currentTaskSubject,omitempty"`
	Activity           string `json:"activity,omitempty"`
	RunningDuration    string `json:"runningDuration,omitempty"`
	Uptime             string `json:"uptime,omitempty"`
}

type teamStatusTaskSummary struct {
	Pending   int `json:"pending"`
	Assigned  int `json:"assigned"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

type teamStatusRecentCompletion struct {
	ID          int    `json:"id"`
	Subject     string `json:"subject"`
	Owner       string `json:"owner,omitempty"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type teamStatusRecentMessage struct {
	From      string `json:"from"`
	To        string `json:"to,omitempty"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	TaskID    int    `json:"taskId,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type teamStatusOutput struct {
	Team              string                      `json:"team"`
	Agents            []teamStatusAgentInfo       `json:"agents"`
	Tasks             teamStatusTaskSummary       `json:"tasks"`
	RecentCompletions []teamStatusRecentCompletion `json:"recentCompletions"`
	RecentMessages    []teamStatusRecentMessage   `json:"recentMessages,omitempty"`
	Notifications     []taskNotification          `json:"pending_notifications,omitempty"`
}

func teamStatusHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamStatusInput) (*mcpsdk.CallToolResult, teamStatusOutput, error) {
	cfg, err := agent.GetTeam(input.Name)
	if err != nil {
		return nil, teamStatusOutput{}, err
	}

	// Agents
	agents := make([]teamStatusAgentInfo, 0, len(cfg.Members))
	for _, m := range cfg.Members {
		info := teamStatusAgentInfo{Name: m.Name}
		alive := agent.IsAgentAlive(input.Name, m.Name)
		info.Alive = alive
		state, _ := agent.GetAgentState(input.Name, m.Name)
		if state != nil {
			info.Status = string(state.Status)
			info.CurrentTask = state.CurrentTask
			info.CurrentTaskSubject = state.CurrentTaskSubject
			info.Activity = state.Activity
			if !state.StartedAt.IsZero() {
				info.Uptime = time.Since(state.StartedAt).Truncate(time.Second).String()
			}
			// Calculate running duration from current task's StartedAt
			if state.CurrentTask > 0 {
				if t, err := agent.GetTask(input.Name, state.CurrentTask); err == nil && t.StartedAt != nil {
					info.RunningDuration = time.Since(*t.StartedAt).Truncate(time.Second).String()
				}
			}
		} else {
			info.Status = "not started"
		}
		agents = append(agents, info)
	}

	// Tasks
	allTasks, _ := agent.ListTasks(input.Name, "", "")
	var summary teamStatusTaskSummary
	var completions []teamStatusRecentCompletion

	for _, t := range allTasks {
		switch t.Status {
		case agent.TaskPending:
			summary.Pending++
		case agent.TaskAssigned:
			summary.Assigned++
		case agent.TaskRunning:
			summary.Running++
		case agent.TaskCompleted:
			summary.Completed++
			cat := ""
			if t.CompletedAt != nil {
				cat = t.CompletedAt.Format("2006-01-02T15:04:05Z")
			}
			completions = append(completions, teamStatusRecentCompletion{
				ID:          t.ID,
				Subject:     t.Subject,
				Owner:       t.Owner,
				CompletedAt: cat,
			})
		case agent.TaskFailed:
			summary.Failed++
		}
	}

	// Only keep last 5 completions
	if len(completions) > 5 {
		completions = completions[len(completions)-5:]
	}

	// Recent messages
	var recentMessages []teamStatusRecentMessage
	if msgs, err := agent.GetAllTeamMessages(input.Name, 10); err == nil {
		for _, msg := range msgs {
			recentMessages = append(recentMessages, teamStatusRecentMessage{
				From:      msg.From,
				To:        msg.To,
				Type:      string(msg.Type),
				Content:   truncateMCP(msg.Content, 200),
				TaskID:    msg.TaskID,
				CreatedAt: msg.CreatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}
	}

	return nil, teamStatusOutput{
		Team:              input.Name,
		Agents:            agents,
		Tasks:             summary,
		RecentCompletions: completions,
		RecentMessages:    recentMessages,
		Notifications:     drainPendingNotifications(),
	}, nil
}

// -- team_start_all --

type teamStartAllInput struct {
	Name string `json:"name" jsonschema:"Team name"`
}

type teamStartAllResult struct {
	Name    string `json:"name"`
	Started bool   `json:"started"`
	PID     int    `json:"pid,omitempty"`
	Error   string `json:"error,omitempty"`
}

type teamStartAllOutput struct {
	Results       []teamStartAllResult `json:"results"`
	MonitorActive bool                 `json:"monitor_active"`
	Notifications []taskNotification   `json:"pending_notifications,omitempty"`
}

func teamStartAllHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamStartAllInput) (*mcpsdk.CallToolResult, teamStartAllOutput, error) {
	agentResults, err := agent.StartAllAgents(input.Name)
	if err != nil {
		return nil, teamStartAllOutput{}, err
	}

	var results []teamStartAllResult
	for _, ar := range agentResults {
		results = append(results, teamStartAllResult{
			Name:    ar.Name,
			Started: ar.Started,
			PID:     ar.PID,
			Error:   ar.Error,
		})
	}

	// Ensure background notification monitor is running
	ensureMonitorRunning(mcpServer)

	return nil, teamStartAllOutput{
		Results:       results,
		MonitorActive: monitorRunning.Load(),
		Notifications: drainPendingNotifications(),
	}, nil
}

// -- team_stop_all --

type teamStopAllInput struct {
	Name string `json:"name" jsonschema:"Team name"`
}

type teamStopAllResult struct {
	Name     string `json:"name"`
	Stopping bool   `json:"stopping"`
	Error    string `json:"error,omitempty"`
}

type teamStopAllOutput struct {
	Results []teamStopAllResult `json:"results"`
}

func teamStopAllHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamStopAllInput) (*mcpsdk.CallToolResult, teamStopAllOutput, error) {
	cfg, err := agent.GetTeam(input.Name)
	if err != nil {
		return nil, teamStopAllOutput{}, err
	}

	var results []teamStopAllResult
	for _, m := range cfg.Members {
		r := teamStopAllResult{Name: m.Name}
		_, err := agent.SendMessage(input.Name, "__system__", m.Name, "__stop__")
		if err != nil {
			r.Error = err.Error()
		} else {
			r.Stopping = true
		}
		results = append(results, r)
	}

	return nil, teamStopAllOutput{Results: results}, nil
}

// registerAgentTools registers all agent-related MCP tools on the given server.
// truncateMCP compresses newlines and truncates a string for MCP output.
func truncateMCP(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// -- team_activity --

type teamActivityInput struct {
	Name  string `json:"name" jsonschema:"Team name"`
	Limit int    `json:"limit,omitempty" jsonschema:"Max events to return (default 20, max 100)"`
}

type activityEvent struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Agent     string `json:"agent"`
	Summary   string `json:"summary"`
	TaskID    int    `json:"taskId,omitempty"`
}

type teamActivityOutput struct {
	Team   string          `json:"team"`
	Events []activityEvent `json:"events"`
}

func teamActivityHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamActivityInput) (*mcpsdk.CallToolResult, teamActivityOutput, error) {
	if input.Name == "" {
		return nil, teamActivityOutput{}, fmt.Errorf("team name is required")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var events []activityEvent

	// Source 1: Messages
	if msgs, err := agent.GetAllTeamMessages(input.Name, 0); err == nil {
		for _, msg := range msgs {
			eventType := "message"
			switch msg.Type {
			case agent.MsgTaskCompleted:
				eventType = "task_completed"
			case agent.MsgTaskFailed:
				eventType = "task_failed"
			case agent.MsgProgress:
				eventType = "progress"
			case agent.MsgHelpRequest:
				eventType = "help_request"
			case agent.MsgDiscovery:
				eventType = "discovery"
			}
			summary := truncateMCP(msg.Content, 150)
			if msg.To != "" {
				summary = fmt.Sprintf("[to %s] %s", msg.To, summary)
			}
			events = append(events, activityEvent{
				Timestamp: msg.CreatedAt.Format("2006-01-02T15:04:05Z"),
				Type:      eventType,
				Agent:     msg.From,
				Summary:   summary,
				TaskID:    msg.TaskID,
			})
		}
	}

	// Source 2: Task lifecycle events
	if tasks, err := agent.ListTasks(input.Name, "", ""); err == nil {
		for _, t := range tasks {
			// Task created
			events = append(events, activityEvent{
				Timestamp: t.CreatedAt.Format("2006-01-02T15:04:05Z"),
				Type:      "task_created",
				Agent:     t.Owner,
				Summary:   fmt.Sprintf("Task #%d created: %s", t.ID, t.Subject),
				TaskID:    t.ID,
			})
			// Task started
			if t.StartedAt != nil {
				events = append(events, activityEvent{
					Timestamp: t.StartedAt.Format("2006-01-02T15:04:05Z"),
					Type:      "task_started",
					Agent:     t.Owner,
					Summary:   fmt.Sprintf("Task #%d started: %s", t.ID, t.Subject),
					TaskID:    t.ID,
				})
			}
			// Task completed
			if t.CompletedAt != nil && t.Status == agent.TaskCompleted {
				events = append(events, activityEvent{
					Timestamp: t.CompletedAt.Format("2006-01-02T15:04:05Z"),
					Type:      "task_completed",
					Agent:     t.Owner,
					Summary:   fmt.Sprintf("Task #%d completed: %s", t.ID, t.Subject),
					TaskID:    t.ID,
				})
			}
			// Task failed
			if t.CompletedAt != nil && t.Status == agent.TaskFailed {
				events = append(events, activityEvent{
					Timestamp: t.CompletedAt.Format("2006-01-02T15:04:05Z"),
					Type:      "task_failed",
					Agent:     t.Owner,
					Summary:   fmt.Sprintf("Task #%d failed: %s", t.ID, t.Subject),
					TaskID:    t.ID,
				})
			}
		}
	}

	// Sort all events by timestamp descending
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp > events[j].Timestamp
	})

	// Apply limit
	if len(events) > limit {
		events = events[:limit]
	}

	return nil, teamActivityOutput{
		Team:   input.Name,
		Events: events,
	}, nil
}

func registerAgentTools(server *mcpsdk.Server) {
	mcpServer = server

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "team_create",
		Description: "Create a new agent team workspace with directories for tasks, messages, and agent state",
	}, teamCreateHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "team_delete",
		Description: "Delete a team and all its data (tasks, messages, agents)",
	}, teamDeleteHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "team_list",
		Description: "List all configured teams",
	}, teamListHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "team_get",
		Description: "Get team configuration and live agent statuses",
	}, teamGetHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "team_status",
		Description: "Get a team dashboard with agent statuses, task summary, and recent completions. Also returns any pending agent notifications.",
	}, teamStatusHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "team_start_all",
		Description: "Start all agent daemons in a team, skipping already running agents. Notifications are piggybacked in subsequent agent tool responses via pending_notifications. RECOMMENDED: after starting, call team_watch and run the returned command in a background Task (run_in_background=true, subagent_type=Bash) for real-time notifications. Also call team_status periodically to check progress.",
	}, teamStartAllHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "team_stop_all",
		Description: "Send stop signals to all agents in a team",
	}, teamStopAllHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "agent_add",
		Description: "Register a new agent in a team",
	}, agentAddHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "agent_remove",
		Description: "Remove an agent from a team",
	}, agentRemoveHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "agent_list",
		Description: "List all agents in a team with their live status. Also returns any pending agent notifications.",
	}, agentListHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "agent_start",
		Description: "Start an agent daemon that polls for and executes tasks. Notifications are piggybacked in subsequent agent tool responses via pending_notifications. RECOMMENDED: after starting, call team_watch and run the returned command in a background Task (run_in_background=true, subagent_type=Bash) for real-time notifications.",
	}, agentStartHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "agent_stop",
		Description: "Stop a running agent daemon gracefully",
	}, agentStopHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task_create",
		Description: "Create a new task in a team, optionally assigning it to an agent. Notifications are piggybacked in subsequent agent tool responses via pending_notifications. After creating tasks, periodically call team_status to check for completion. For real-time monitoring, call team_watch and run the returned command in a background Task.",
	}, taskCreateHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task_update",
		Description: "Update task fields including status, owner, result, or description",
	}, taskUpdateHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task_redirect",
		Description: "Cancel a running task and create a new one with updated instructions. The new task inherits the original task's owner, priority, project, and working directory. The agent daemon will automatically detect the cancellation (within ~3 seconds), terminate the running Claude subprocess, and pick up the new task.",
	}, taskRedirectHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task_list",
		Description: "List tasks in a team with optional status and owner filters. Also returns any pending agent notifications.",
	}, taskListHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task_get",
		Description: "Get full details of a specific task including result and session info. Also returns any pending agent notifications.",
	}, taskGetHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "message_send",
		Description: "Send a message from one agent to another, or broadcast to all agents",
	}, messageSendHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "message_list",
		Description: "List messages for an agent, with optional type and unread filters. Use this to read task completion reports and agent responses.",
	}, messageListHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "message_mark_read",
		Description: "Mark a specific message as read",
	}, messageMarkReadHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "test_sampling",
		Description: "Test MCP sampling: send a createMessage request back to the client to verify sampling support",
	}, testSamplingHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "test_progress",
		Description: "Test MCP progress notifications: checks if client sends a progress token and attempts to send progress notifications back. Use this to verify if real-time progress updates work.",
	}, testProgressHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "team_watch",
		Description: `Get a bash command that monitors agent task completion notifications in real time.

RECOMMENDED USAGE — run BOTH steps after starting agents:

Step 1 (bash log): Call team_watch, then run the returned command in a background Bash Task:
  Task(subagent_type="Bash", run_in_background=True, prompt="Run: <command>")
  → Writes notifications to an output file. You must actively read that file to see results.
  → Does NOT automatically wake the main session.

Step 2 (auto-wakeup): ALSO spawn a background general-purpose Task that calls team_subscribe:
  Task(subagent_type="general-purpose", run_in_background=True,
       prompt="Call team_subscribe for team '<name>' and report when any task notification arrives.")
  → Blocks until a notification arrives, then exits and triggers a <task-notification> that
    automatically wakes the main session — no polling needed.

Use both together: bash watch for the running log, team_subscribe for automatic wakeup.`,
	}, teamWatchHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "team_subscribe",
		Description: `Subscribe to agent task notifications for a team. This tool BLOCKS until a notification arrives or timeout is reached.

HOW IT WORKS:
- Call this from a background general-purpose Task (NOT from the main session, NOT from a Bash Task).
- The agent blocks waiting; when a task completes/fails, the tool returns.
- The background agent then exits, automatically triggering a <task-notification> that wakes the main session.
- No polling required — this is event-driven and instant.

CORRECT USAGE (after starting agents):
  Task(subagent_type="general-purpose", run_in_background=True,
       prompt="Call team_subscribe for team '<name>' with timeout=30 and report notifications.")

WRONG USAGE:
  ❌ Calling team_subscribe directly in the main session — blocks everything
  ❌ Running inside a Bash Task — Bash cannot call MCP tools

IMPORTANT: Also run team_watch in a background Bash Task to capture the full notification log.
Prefer this over team_watch alone for automatic main-session wakeup.`,
	}, teamSubscribeHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "team_activity",
		Description: "Get a unified activity timeline for a team, combining messages and task lifecycle events. Returns events sorted by time (newest first). Use limit parameter to control how many events to return (default 20, max 100).",
	}, teamActivityHandler)
}

// -- test_sampling --

type testSamplingInput struct {
	Message string `json:"message" jsonschema:"Message to send via sampling"`
}

type testSamplingOutput struct {
	Supported bool   `json:"supported"`
	Response  string `json:"response,omitempty"`
	Error     string `json:"error,omitempty"`
}

func testSamplingHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input testSamplingInput) (*mcpsdk.CallToolResult, testSamplingOutput, error) {
	if input.Message == "" {
		input.Message = "Agent notification test: task completed successfully"
	}

	result, err := req.Session.CreateMessage(ctx, &mcpsdk.CreateMessageParams{
		Messages: []*mcpsdk.SamplingMessage{
			{
				Role:    mcpsdk.Role("user"),
				Content: &mcpsdk.TextContent{Text: input.Message},
			},
		},
		MaxTokens: 200,
	})
	if err != nil {
		return nil, testSamplingOutput{
			Supported: false,
			Error:     err.Error(),
		}, nil
	}

	var responseText string
	if tc, ok := result.Content.(*mcpsdk.TextContent); ok {
		responseText = tc.Text
	}

	return nil, testSamplingOutput{
		Supported: true,
		Response:  responseText,
	}, nil
}

// -- test_progress --

type testProgressInput struct {
	Steps   int    `json:"steps,omitempty" jsonschema:"Number of progress steps to send (default 5)"`
	Message string `json:"message,omitempty" jsonschema:"Custom message prefix for progress notifications"`
}

type testProgressOutput struct {
	HasProgressToken bool   `json:"has_progress_token"`
	ProgressTokenRaw string `json:"progress_token_raw,omitempty"`
	StepsSent        int    `json:"steps_sent"`
	Errors           []string `json:"errors,omitempty"`
	HasSession       bool   `json:"has_session"`
}

func testProgressHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input testProgressInput) (*mcpsdk.CallToolResult, testProgressOutput, error) {
	steps := input.Steps
	if steps <= 0 {
		steps = 5
	}
	if steps > 20 {
		steps = 20
	}
	msgPrefix := input.Message
	if msgPrefix == "" {
		msgPrefix = "Test progress step"
	}

	output := testProgressOutput{
		HasSession: req.Session != nil,
	}

	// Check for progress token
	progressToken := req.Params.GetProgressToken()
	output.HasProgressToken = progressToken != nil
	if progressToken != nil {
		output.ProgressTokenRaw = fmt.Sprintf("%v (type: %T)", progressToken, progressToken)
	}

	// Attempt to send progress notifications
	if progressToken != nil && req.Session != nil {
		for i := 1; i <= steps; i++ {
			err := req.Session.NotifyProgress(ctx, &mcpsdk.ProgressNotificationParams{
				ProgressToken: progressToken,
				Progress:      float64(i),
				Total:         float64(steps),
				Message:       fmt.Sprintf("%s %d/%d", msgPrefix, i, steps),
			})
			if err != nil {
				output.Errors = append(output.Errors, fmt.Sprintf("step %d: %v", i, err))
			} else {
				output.StepsSent++
			}
			// Small delay between steps to allow observation
			time.Sleep(500 * time.Millisecond)
		}
	} else {
		// No progress token — still try sending without token to see what happens
		if req.Session != nil {
			err := req.Session.NotifyProgress(ctx, &mcpsdk.ProgressNotificationParams{
				Progress: 1,
				Total:    1,
				Message:  "test notification without progress token",
			})
			if err != nil {
				output.Errors = append(output.Errors, fmt.Sprintf("no-token attempt: %v", err))
			}
		}
	}

	return nil, output, nil
}

// -- team_watch --

type teamWatchInput struct {
	Team    string `json:"team,omitempty" jsonschema:"Optional team name to filter notifications"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"Monitoring duration in minutes (default 30)"`
}

type teamWatchOutput struct {
	Command     string `json:"command" jsonschema:"Bash command to run in a background Task for monitoring"`
	Instruction string `json:"instruction" jsonschema:"How to use this command"`
}

func teamWatchHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamWatchInput) (*mcpsdk.CallToolResult, teamWatchOutput, error) {
	timeout := input.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	iterations := timeout * 12 // 5-second intervals

	// Use find instead of ls to avoid TOCTOU races and handle filenames with spaces.
	// The __ separator matches the updated writeNotification filename format.
	filter := "*.json"
	if input.Team != "" {
		filter = fmt.Sprintf("%s__*.json", input.Team)
	}

	cmd := fmt.Sprintf(`mkdir -p ~/.codes/notifications && echo "Monitoring agent notifications (timeout: %dm)..." && for i in $(seq 1 %d); do found=0; for f in $(find ~/.codes/notifications -maxdepth 1 -name '%s' -type f 2>/dev/null); do echo "=== Agent Notification ==="; cat "$f" && rm -f "$f"; echo ""; found=1; done; sleep 5; done && echo "Monitor timeout reached"`, timeout, iterations, filter)

	return nil, teamWatchOutput{
		Command:     cmd,
		Instruction: "Run this command using the Task tool with run_in_background=true and subagent_type=Bash. You will be automatically notified when agents complete tasks.",
	}, nil
}

// -- team_subscribe --

type teamSubscribeInput struct {
	Team    string `json:"team" jsonschema:"Team name to subscribe to"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"Max wait time in minutes (default 10)"`
}

type teamSubscribeOutput struct {
	Notifications []taskNotification `json:"notifications"`
	TimedOut      bool               `json:"timed_out"`
	Team          string             `json:"team"`
}

func teamSubscribeHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamSubscribeInput) (*mcpsdk.CallToolResult, teamSubscribeOutput, error) {
	if input.Team == "" {
		return nil, teamSubscribeOutput{}, fmt.Errorf("team name is required")
	}

	timeout := input.Timeout
	if timeout <= 0 {
		timeout = 10
	}

	ensureMonitorRunning(mcpServer)

	d := time.Duration(timeout) * time.Minute
	if subscribeTimeoutOverride > 0 {
		d = subscribeTimeoutOverride
	}
	deadline := time.Now().Add(d)

	// Use notifCond to wait efficiently for new notifications.
	// The cond is based on pendingMu, so we hold the lock during Wait.
	pendingMu.Lock()
	for {
		matched := drainTeamNotificationsLocked(input.Team)
		if len(matched) > 0 {
			// Return immediately so the background agent exits and triggers
			// a <task-notification> to the main session.
			pendingMu.Unlock()
			return nil, teamSubscribeOutput{
				Notifications: matched,
				TimedOut:      false,
				Team:          input.Team,
			}, nil
		}

		// Check context cancellation.
		select {
		case <-ctx.Done():
			pendingMu.Unlock()
			return nil, teamSubscribeOutput{
				TimedOut: true,
				Team:     input.Team,
			}, nil
		default:
		}

		// Check deadline.
		if time.Now().After(deadline) {
			pendingMu.Unlock()
			return nil, teamSubscribeOutput{
				TimedOut: true,
				Team:     input.Team,
			}, nil
		}

		// Wait with a periodic wake-up to re-check deadline/ctx.
		// notifCond.Wait() releases pendingMu and re-acquires on wake.
		// Use a goroutine to impose a wake-up cap so we don't block forever
		// if no notifications arrive.
		wakeUp := make(chan struct{}, 1)
		go func() {
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
			}
			// Broadcast to wake up all waiters so they can re-check.
			notifCond.Broadcast()
			select {
			case wakeUp <- struct{}{}:
			default:
			}
		}()

		notifCond.Wait()
		// Drain the wake-up channel to avoid goroutine leak.
		select {
		case <-wakeUp:
		default:
		}
	}
}
