package mcpserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"codes/internal/agent"
)

const notificationMonitorCmd = `mkdir -p ~/.codes/notifications && echo "Monitoring agent notifications..." && for i in $(seq 1 360); do files=$(ls ~/.codes/notifications/*.json 2>/dev/null); if [ -n "$files" ]; then echo "=== Agent Notification ==="; for f in $files; do cat "$f"; echo ""; rm "$f"; done; i=0; fi; sleep 5; done`

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
	Agents []agentInfo `json:"agents"`
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

	return nil, agentListOutput{Agents: agents}, nil
}

// -- agent_start --

type agentStartInput struct {
	Team string `json:"team" jsonschema:"Team name"`
	Name string `json:"name" jsonschema:"Agent name to start"`
}

type agentStartOutput struct {
	Started    bool   `json:"started"`
	PID        int    `json:"pid,omitempty"`
	MonitorCmd string `json:"monitor_cmd" jsonschema:"Bash command to monitor agent notifications. Run this in a background Task to receive completion alerts."`
}

func agentStartHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input agentStartInput) (*mcpsdk.CallToolResult, agentStartOutput, error) {
	// Verify the agent exists before spawning
	_, err := agent.NewDaemon(input.Team, input.Name)
	if err != nil {
		return nil, agentStartOutput{}, err
	}

	// Check if the agent is already alive
	if agent.IsAgentAlive(input.Team, input.Name) {
		state, _ := agent.GetAgentState(input.Team, input.Name)
		pid := 0
		if state != nil {
			pid = state.PID
		}
		return nil, agentStartOutput{}, fmt.Errorf("agent %q is already running (pid %d)", input.Name, pid)
	}

	// Spawn as independent subprocess so it survives MCP server restarts
	exe, err := os.Executable()
	if err != nil {
		return nil, agentStartOutput{}, fmt.Errorf("cannot find executable: %w", err)
	}

	cmd := exec.Command(exe, "agent", "run", input.Team, input.Name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, agentStartOutput{}, fmt.Errorf("failed to start agent: %w", err)
	}

	pid := cmd.Process.Pid
	cmd.Process.Release() // detach

	return nil, agentStartOutput{Started: true, PID: pid, MonitorCmd: notificationMonitorCmd}, nil
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
	Task *agent.Task `json:"task"`
}

func taskCreateHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input taskCreateInput) (*mcpsdk.CallToolResult, taskCreateOutput, error) {
	if input.Team == "" || input.Subject == "" {
		return nil, taskCreateOutput{}, fmt.Errorf("team and subject are required")
	}
	task, err := agent.CreateTask(input.Team, input.Subject, input.Description, input.Assign, input.BlockedBy, agent.TaskPriority(input.Priority), input.Project, input.WorkDir)
	if err != nil {
		return nil, taskCreateOutput{}, err
	}
	return nil, taskCreateOutput{Task: task}, nil
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

// -- task_list --

type taskListInput struct {
	Team   string `json:"team" jsonschema:"Team name"`
	Status string `json:"status,omitempty" jsonschema:"Filter by status"`
	Owner  string `json:"owner,omitempty" jsonschema:"Filter by owner"`
}

type taskListOutput struct {
	Tasks []*agent.Task `json:"tasks"`
}

func taskListHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input taskListInput) (*mcpsdk.CallToolResult, taskListOutput, error) {
	tasks, err := agent.ListTasks(input.Team, agent.TaskStatus(input.Status), input.Owner)
	if err != nil {
		return nil, taskListOutput{}, err
	}
	if tasks == nil {
		tasks = []*agent.Task{}
	}
	return nil, taskListOutput{Tasks: tasks}, nil
}

// -- task_get --

type taskGetInput struct {
	Team   string `json:"team" jsonschema:"Team name"`
	TaskID int    `json:"taskId" jsonschema:"Task ID"`
}

type taskGetOutput struct {
	Task *agent.Task `json:"task"`
}

func taskGetHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input taskGetInput) (*mcpsdk.CallToolResult, taskGetOutput, error) {
	task, err := agent.GetTask(input.Team, input.TaskID)
	if err != nil {
		return nil, taskGetOutput{}, err
	}
	return nil, taskGetOutput{Task: task}, nil
}

// -- message_send --

type messageSendInput struct {
	Team    string `json:"team" jsonschema:"Team name"`
	From    string `json:"from" jsonschema:"Sender agent name"`
	To      string `json:"to,omitempty" jsonschema:"Recipient agent name (empty for broadcast)"`
	Content string `json:"content" jsonschema:"Message content"`
}

type messageSendOutput struct {
	Message *agent.Message `json:"message"`
}

func messageSendHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input messageSendInput) (*mcpsdk.CallToolResult, messageSendOutput, error) {
	if input.Team == "" || input.From == "" || input.Content == "" {
		return nil, messageSendOutput{}, fmt.Errorf("team, from, and content are required")
	}
	msg, err := agent.SendMessage(input.Team, input.From, input.To, input.Content)
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
	Name        string `json:"name"`
	Status      string `json:"status"`
	Alive       bool   `json:"alive"`
	CurrentTask int    `json:"currentTask,omitempty"`
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

type teamStatusOutput struct {
	Team              string                       `json:"team"`
	Agents            []teamStatusAgentInfo        `json:"agents"`
	Tasks             teamStatusTaskSummary        `json:"tasks"`
	RecentCompletions []teamStatusRecentCompletion `json:"recentCompletions"`
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

	return nil, teamStatusOutput{
		Team:              input.Name,
		Agents:            agents,
		Tasks:             summary,
		RecentCompletions: completions,
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
	Results    []teamStartAllResult `json:"results"`
	MonitorCmd string               `json:"monitor_cmd" jsonschema:"Bash command to monitor agent notifications. Run this in a background Task to receive completion alerts."`
}

func teamStartAllHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input teamStartAllInput) (*mcpsdk.CallToolResult, teamStartAllOutput, error) {
	cfg, err := agent.GetTeam(input.Name)
	if err != nil {
		return nil, teamStartAllOutput{}, err
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, teamStartAllOutput{}, fmt.Errorf("cannot find executable: %w", err)
	}

	var results []teamStartAllResult
	for _, m := range cfg.Members {
		r := teamStartAllResult{Name: m.Name}

		// Skip already alive agents
		if agent.IsAgentAlive(input.Name, m.Name) {
			state, _ := agent.GetAgentState(input.Name, m.Name)
			if state != nil {
				r.PID = state.PID
			}
			r.Error = "already running"
			results = append(results, r)
			continue
		}

		cmd := exec.Command(exe, "agent", "run", input.Name, m.Name)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			r.Error = err.Error()
			results = append(results, r)
			continue
		}

		r.Started = true
		r.PID = cmd.Process.Pid
		cmd.Process.Release()
		results = append(results, r)
	}

	return nil, teamStartAllOutput{Results: results, MonitorCmd: notificationMonitorCmd}, nil
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
func registerAgentTools(server *mcpsdk.Server) {
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
		Description: "Get a team dashboard with agent statuses, task summary, and recent completions",
	}, teamStatusHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "team_start_all",
		Description: "Start all agent daemons in a team, skipping already running agents. IMPORTANT: After starting, launch a background monitor to receive task completion notifications. Use the Task tool (run_in_background=true, subagent_type=Bash) with a bash script that polls ~/.codes/notifications/ for .json files. When a file appears, read and report its contents, then delete it. This enables real-time agent progress reporting to the current session.",
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
		Description: "List all agents in a team with their live status",
	}, agentListHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "agent_start",
		Description: "Start an agent daemon that polls for and executes tasks. After starting, use a background task to monitor ~/.codes/notifications/ for completion notifications (e.g. Task tool with run_in_background + bash polling loop).",
	}, agentStartHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "agent_stop",
		Description: "Stop a running agent daemon gracefully",
	}, agentStopHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task_create",
		Description: "Create a new task in a team, optionally assigning it to an agent",
	}, taskCreateHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task_update",
		Description: "Update task fields including status, owner, result, or description",
	}, taskUpdateHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task_list",
		Description: "List tasks in a team with optional status and owner filters",
	}, taskListHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task_get",
		Description: "Get full details of a specific task including result and session info",
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
		Name:        "team_watch",
		Description: "Get a bash command that monitors agent task completion notifications. Run the returned command in a background Task (run_in_background=true, subagent_type=Bash) to receive real-time notifications when agents complete or fail tasks. The command polls ~/.codes/notifications/ for JSON files written by agent daemons.",
	}, teamWatchHandler)
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

	cmd := fmt.Sprintf(`mkdir -p ~/.codes/notifications && echo "Monitoring agent notifications (timeout: %dm)..." && for i in $(seq 1 %d); do files=$(ls ~/.codes/notifications/*.json 2>/dev/null); if [ -n "$files" ]; then echo "=== Agent Notification ==="; for f in $files; do cat "$f"; echo ""; rm "$f"; done; fi; sleep 5; done && echo "Monitor timeout reached"`, timeout, iterations)

	return nil, teamWatchOutput{
		Command:     cmd,
		Instruction: "Run this command using the Task tool with run_in_background=true and subagent_type=Bash. You will be automatically notified when agents complete tasks.",
	}, nil
}
