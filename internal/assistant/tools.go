package assistant

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/toolrunner"

	"codes/internal/agent"
	"codes/internal/assistant/memory"
	"codes/internal/assistant/scheduler"
	"codes/internal/config"
)

// globalScheduler is injected via SetScheduler so tools can call Reload after mutations.
var globalScheduler *scheduler.Scheduler

// SetScheduler injects the runtime Scheduler so tool handlers can call Reload.
func SetScheduler(s *scheduler.Scheduler) {
	globalScheduler = s
}

// taskDef is a single task to be dispatched to a worker agent.
type taskDef struct {
	Subject     string `json:"subject" jsonschema:"required,description=Brief task title"`
	Description string `json:"description" jsonschema:"required,description=Detailed task description for the coding agent"`
	DependsOn   []int  `json:"depends_on,omitempty" jsonschema:"description=1-based indices of tasks this must wait for"`
}

// toolText is a convenience helper to return a plain text tool result.
func toolText(s string) anthropic.BetaToolResultBlockParamContentUnion {
	return anthropic.BetaToolResultBlockParamContentUnion{
		OfText: &anthropic.BetaTextBlockParam{Text: s},
	}
}

func generateTeamName() string {
	buf := make([]byte, 2)
	_, _ = rand.Read(buf)
	suffix := int(buf[0])<<8 | int(buf[1])
	return fmt.Sprintf("assistant-%d-%04d", time.Now().UnixNano(), suffix%10000)
}

// buildTools constructs all tools the assistant can use.
func buildTools() ([]anthropic.BetaTool, error) {
	// -- list_projects --
	type listProjectsInput struct{}
	listProjectsTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"list_projects",
		"List all registered projects with their paths.",
		func(ctx context.Context, _ listProjectsInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			projects, err := config.ListProjects()
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			if len(projects) == 0 {
				return toolText("No projects registered."), nil
			}
			out := "Available projects:\n"
			for name, entry := range projects {
				out += fmt.Sprintf("  - %s: %s\n", name, entry.Path)
			}
			return toolText(out), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list_projects tool: %w", err)
	}

	// -- run_tasks --
	type runTasksInput struct {
		Project string    `json:"project" jsonschema:"required,description=Project name to work in"`
		Tasks   []taskDef `json:"tasks" jsonschema:"required,description=List of tasks to create and execute"`
	}
	runTasksTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"run_tasks",
		"Create an agent team and execute one or more coding tasks in a project. Tasks run in parallel by default; use depends_on for sequential ordering.",
		func(ctx context.Context, input runTasksInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			project, exists := config.GetProject(input.Project)
			if !exists {
				return toolText(fmt.Sprintf("project %q not found. Call list_projects to see available projects.", input.Project)), nil
			}
			if len(input.Tasks) == 0 {
				return toolText("no tasks provided"), nil
			}

			teamName, err := dispatchTasks(input.Project, input.Tasks, project.Path)
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			return toolText(fmt.Sprintf("Team %q created with %d task(s). Call get_team_status to monitor progress.", teamName, len(input.Tasks))), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("run_tasks tool: %w", err)
	}

	// -- get_team_status --
	type getTeamStatusInput struct {
		Team string `json:"team" jsonschema:"required,description=Team name returned by run_tasks"`
	}
	getTeamStatusTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"get_team_status",
		"Get the current status of an agent team and its tasks.",
		func(ctx context.Context, input getTeamStatusInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			tasks, err := agent.ListTasks(input.Team, "", "")
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			if len(tasks) == 0 {
				return toolText("no tasks found in team " + input.Team), nil
			}
			out := fmt.Sprintf("Team %q — %d task(s):\n", input.Team, len(tasks))
			for _, t := range tasks {
				out += fmt.Sprintf("  [%s] #%d %s\n", t.Status, t.ID, t.Subject)
				if t.Result != "" {
					r := t.Result
					if len(r) > 200 {
						r = r[:200] + "..."
					}
					out += fmt.Sprintf("         result: %s\n", r)
				}
			}
			return toolText(out), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("get_team_status tool: %w", err)
	}

	// -- list_teams --
	type listTeamsInput struct{}
	listTeamsTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"list_teams",
		"List all active agent teams.",
		func(ctx context.Context, _ listTeamsInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			names, err := agent.ListTeams()
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			if len(names) == 0 {
				return toolText("No active teams."), nil
			}
			out := fmt.Sprintf("%d active team(s):\n", len(names))
			for _, name := range names {
				out += "  - " + name + "\n"
			}
			return toolText(out), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list_teams tool: %w", err)
	}

	// ── Memory tools ──────────────────────────────────────────────────────────

	// -- remember --
	type rememberInput struct {
		Name         string   `json:"name" jsonschema:"required,description=Entity name (e.g. 'User', 'codes project')"`
		EntityType   string   `json:"entity_type" jsonschema:"required,description=Type: person/project/preference/note/event"`
		Observations []string `json:"observations" jsonschema:"required,description=Facts to store or append"`
	}
	rememberTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"remember",
		"Create a memory entity or append observations to an existing one. Use this whenever you learn something worth remembering about the user or their projects.",
		func(ctx context.Context, input rememberInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			// Try to append to existing entity first.
			err := memory.AddObservations(input.Name, input.Observations)
			if err != nil {
				// Entity doesn't exist — create it.
				createErr := memory.CreateEntities([]memory.Entity{{
					Name:         input.Name,
					EntityType:   input.EntityType,
					Observations: input.Observations,
				}})
				if createErr != nil {
					return toolText("error: " + createErr.Error()), nil
				}
				return toolText(fmt.Sprintf("Created entity %q (%s) with %d observation(s).", input.Name, input.EntityType, len(input.Observations))), nil
			}
			return toolText(fmt.Sprintf("Appended %d observation(s) to entity %q.", len(input.Observations), input.Name)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("remember tool: %w", err)
	}

	// -- recall --
	type recallInput struct {
		Query string `json:"query" jsonschema:"required,description=Search query (case-insensitive substring match)"`
	}
	recallTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"recall",
		"Search memories by keyword. Returns matching entities and their observations.",
		func(ctx context.Context, input recallInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			entities, err := memory.SearchNodes(input.Query)
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			if len(entities) == 0 {
				return toolText(fmt.Sprintf("No memories found matching %q.", input.Query)), nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Found %d entity/entities matching %q:\n", len(entities), input.Query))
			for _, e := range entities {
				sb.WriteString(fmt.Sprintf("\n[%s] %s\n", e.EntityType, e.Name))
				for _, o := range e.Observations {
					sb.WriteString("  - " + o + "\n")
				}
			}
			return toolText(sb.String()), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("recall tool: %w", err)
	}

	// -- forget --
	type forgetInput struct {
		Name string `json:"name" jsonschema:"required,description=Entity name to delete"`
	}
	forgetTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"forget",
		"Delete a memory entity and all its relations by name.",
		func(ctx context.Context, input forgetInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			if err := memory.DeleteEntity(input.Name); err != nil {
				return toolText("error: " + err.Error()), nil
			}
			return toolText(fmt.Sprintf("Deleted entity %q.", input.Name)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("forget tool: %w", err)
	}

	// -- update_profile --
	type updateProfileInput struct {
		Field string `json:"field" jsonschema:"required,description=Field: name/timezone/language/default_project/notes"`
		Value string `json:"value" jsonschema:"required,description=New value"`
	}
	updateProfileTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"update_profile",
		"Update a field in the user profile. Valid fields: name, timezone, language, default_project, notes.",
		func(ctx context.Context, input updateProfileInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			if err := memory.UpdateProfile(input.Field, input.Value); err != nil {
				return toolText("error: " + err.Error()), nil
			}
			return toolText(fmt.Sprintf("Updated profile field %q to %q.", input.Field, input.Value)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("update_profile tool: %w", err)
	}

	// ── Scheduler tools ───────────────────────────────────────────────────────

	// -- set_reminder --
	type setReminderInput struct {
		Message   string `json:"message" jsonschema:"required,description=Message to send when reminder fires"`
		At        string `json:"at" jsonschema:"required,description=ISO 8601 datetime e.g. 2026-02-21T09:00:00+08:00"`
		SessionID string `json:"session_id,omitempty" jsonschema:"description=Session to deliver to (default: same session)"`
	}
	setReminderTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"set_reminder",
		"Set a one-time reminder. Fires at the specified datetime and delivers the message to the assistant session.",
		func(ctx context.Context, input setReminderInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			t, err := time.Parse(time.RFC3339, input.At)
			if err != nil {
				return toolText("error: invalid 'at' format — use ISO 8601 e.g. 2026-02-21T09:00:00+08:00"), nil
			}
			sid := input.SessionID
			if sid == "" {
				sid = "default"
			}
			s := &scheduler.Schedule{
				Type:      scheduler.TypeOnce,
				Message:   input.Message,
				SessionID: sid,
				At:        &t,
				Enabled:   true,
			}
			if err := scheduler.AddSchedule(s); err != nil {
				return toolText("error: " + err.Error()), nil
			}
			if globalScheduler != nil {
				_ = globalScheduler.Reload()
			}
			return toolText(fmt.Sprintf("Reminder set (id=%s) for %s.", s.ID, t.Format(time.RFC3339))), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("set_reminder tool: %w", err)
	}

	// -- set_schedule --
	type setScheduleInput struct {
		Message   string `json:"message" jsonschema:"required,description=Message to send on each trigger"`
		Cron      string `json:"cron" jsonschema:"required,description=Cron expression e.g. '0 9 * * *' for 9am daily"`
		SessionID string `json:"session_id,omitempty" jsonschema:"description=Session to deliver to (default: same session)"`
	}
	setScheduleTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"set_schedule",
		"Set a recurring schedule using a cron expression. Delivers the message to the assistant session on each trigger.",
		func(ctx context.Context, input setScheduleInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			sid := input.SessionID
			if sid == "" {
				sid = "default"
			}
			s := &scheduler.Schedule{
				Type:      scheduler.TypePeriodic,
				Message:   input.Message,
				SessionID: sid,
				Cron:      input.Cron,
				Enabled:   true,
			}
			if err := scheduler.AddSchedule(s); err != nil {
				return toolText("error: " + err.Error()), nil
			}
			if globalScheduler != nil {
				_ = globalScheduler.Reload()
			}
			return toolText(fmt.Sprintf("Periodic schedule created (id=%s) with cron=%q.", s.ID, input.Cron)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("set_schedule tool: %w", err)
	}

	// -- list_schedules --
	type listSchedulesInput struct{}
	listSchedulesTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"list_schedules",
		"List all scheduled reminders and periodic tasks.",
		func(ctx context.Context, _ listSchedulesInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			schedules, err := scheduler.ListSchedules()
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			if len(schedules) == 0 {
				return toolText("No schedules configured."), nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%d schedule(s):\n", len(schedules)))
			for _, s := range schedules {
				enabled := "enabled"
				if !s.Enabled {
					enabled = "disabled"
				}
				switch s.Type {
				case scheduler.TypeOnce:
					when := "(no time)"
					if s.At != nil {
						when = s.At.Format(time.RFC3339)
					}
					sb.WriteString(fmt.Sprintf("  [%s] %s | once at %s | session=%s | %q\n",
						enabled, s.ID, when, s.SessionID, s.Message))
				case scheduler.TypePeriodic:
					sb.WriteString(fmt.Sprintf("  [%s] %s | cron=%q | session=%s | %q\n",
						enabled, s.ID, s.Cron, s.SessionID, s.Message))
				}
			}
			return toolText(sb.String()), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list_schedules tool: %w", err)
	}

	// -- cancel_schedule --
	type cancelScheduleInput struct {
		ID string `json:"id" jsonschema:"required,description=Schedule ID from list_schedules"`
	}
	cancelScheduleTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"cancel_schedule",
		"Cancel and remove a scheduled reminder or periodic task by ID.",
		func(ctx context.Context, input cancelScheduleInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			if err := scheduler.RemoveSchedule(input.ID); err != nil {
				return toolText("error: " + err.Error()), nil
			}
			if globalScheduler != nil {
				_ = globalScheduler.Reload()
			}
			return toolText(fmt.Sprintf("Schedule %q cancelled.", input.ID)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cancel_schedule tool: %w", err)
	}

	// ── Team control tools ───────────────────────────────────────────────────

	// -- stop_agent --
	type stopAgentInput struct {
		Team  string `json:"team" jsonschema:"required,description=Team name"`
		Agent string `json:"agent" jsonschema:"required,description=Agent name to stop"`
	}
	stopAgentTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"stop_agent",
		"Send a stop signal to a running agent. The agent will finish its current step and exit gracefully.",
		func(ctx context.Context, input stopAgentInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			_, err := agent.SendMessage(input.Team, "assistant", input.Agent, "__stop__")
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			return toolText(fmt.Sprintf("Stop signal sent to agent %q in team %q.", input.Agent, input.Team)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("stop_agent tool: %w", err)
	}

	// -- stop_all_agents --
	type stopAllAgentsInput struct {
		Team string `json:"team" jsonschema:"required,description=Team name"`
	}
	stopAllAgentsTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"stop_all_agents",
		"Send a stop signal to all running agents in a team.",
		func(ctx context.Context, input stopAllAgentsInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			team, err := agent.GetTeam(input.Team)
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			count := 0
			for _, m := range team.Members {
				if _, err := agent.SendMessage(input.Team, "assistant", m.Name, "__stop__"); err == nil {
					count++
				}
			}
			return toolText(fmt.Sprintf("Stop signal sent to %d agent(s) in team %q.", count, input.Team)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("stop_all_agents tool: %w", err)
	}

	// -- delete_team --
	type deleteTeamInput struct {
		Team string `json:"team" jsonschema:"required,description=Team name to delete"`
	}
	deleteTeamTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"delete_team",
		"Delete a team and all its state (tasks, messages, agent files). Stop agents first.",
		func(ctx context.Context, input deleteTeamInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			if err := agent.DeleteTeam(input.Team); err != nil {
				return toolText("error: " + err.Error()), nil
			}
			return toolText(fmt.Sprintf("Team %q deleted.", input.Team)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("delete_team tool: %w", err)
	}

	// -- cancel_task --
	type cancelTaskInput struct {
		Team   string `json:"team" jsonschema:"required,description=Team name"`
		TaskID int    `json:"task_id" jsonschema:"required,description=Task ID to cancel"`
	}
	cancelTaskTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"cancel_task",
		"Cancel a pending or in-progress task. The agent will stop working on it at the next checkpoint.",
		func(ctx context.Context, input cancelTaskInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			task, err := agent.CancelTask(input.Team, input.TaskID)
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			return toolText(fmt.Sprintf("Task #%d %q cancelled.", task.ID, task.Subject)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cancel_task tool: %w", err)
	}

	// -- redirect_task --
	type redirectTaskInput struct {
		Team            string `json:"team" jsonschema:"required,description=Team name"`
		TaskID          int    `json:"task_id" jsonschema:"required,description=Task ID to redirect"`
		NewInstructions string `json:"new_instructions" jsonschema:"required,description=Updated task description"`
		NewSubject      string `json:"new_subject,omitempty" jsonschema:"description=New task title (optional, keeps original if empty)"`
	}
	redirectTaskTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"redirect_task",
		"Cancel a task and create a new one with updated instructions, assigned to the same agent. Use when a task has gone in the wrong direction.",
		func(ctx context.Context, input redirectTaskInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			newTask, err := agent.RedirectTask(input.Team, input.TaskID, input.NewInstructions, input.NewSubject)
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			return toolText(fmt.Sprintf("Task #%d redirected → new task #%d %q.", input.TaskID, newTask.ID, newTask.Subject)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("redirect_task tool: %w", err)
	}

	// -- send_message --
	type sendMessageInput struct {
		Team    string `json:"team" jsonschema:"required,description=Team name"`
		To      string `json:"to" jsonschema:"required,description=Recipient agent name (use broadcast to send to all)"`
		Content string `json:"content" jsonschema:"required,description=Message content"`
	}
	sendMessageTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"send_message",
		"Send a message to a specific agent (or broadcast to all). Agents read messages between task steps.",
		func(ctx context.Context, input sendMessageInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			var err error
			if input.To == "broadcast" || input.To == "" {
				_, err = agent.BroadcastMessage(input.Team, "assistant", input.Content)
			} else {
				_, err = agent.SendMessage(input.Team, "assistant", input.To, input.Content)
			}
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			return toolText(fmt.Sprintf("Message sent to %q in team %q.", input.To, input.Team)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("send_message tool: %w", err)
	}

	// -- add_task --
	type addTaskInput struct {
		Team        string `json:"team" jsonschema:"required,description=Team name"`
		Subject     string `json:"subject" jsonschema:"required,description=Task title"`
		Description string `json:"description" jsonschema:"required,description=Detailed task description"`
		Assign      string `json:"assign,omitempty" jsonschema:"description=Agent name to assign to (first available if empty)"`
	}
	addTaskTool, err := toolrunner.NewBetaToolFromJSONSchema(
		"add_task",
		"Add a new task to an existing team. Useful for injecting follow-up work after reviewing progress.",
		func(ctx context.Context, input addTaskInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			task, err := agent.CreateTask(input.Team, input.Subject, input.Description, input.Assign, nil, agent.PriorityNormal, "", "")
			if err != nil {
				return toolText("error: " + err.Error()), nil
			}
			return toolText(fmt.Sprintf("Task #%d %q added to team %q (assigned: %q).", task.ID, task.Subject, input.Team, task.Owner)), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("add_task tool: %w", err)
	}

	return []anthropic.BetaTool{
		listProjectsTool,
		runTasksTool,
		getTeamStatusTool,
		listTeamsTool,
		stopAgentTool,
		stopAllAgentsTool,
		deleteTeamTool,
		cancelTaskTool,
		redirectTaskTool,
		sendMessageTool,
		addTaskTool,
		rememberTool,
		recallTool,
		forgetTool,
		updateProfileTool,
		setReminderTool,
		setScheduleTool,
		listSchedulesTool,
		cancelScheduleTool,
	}, nil
}

// dispatchTasks creates a team, adds workers, creates tasks, and starts all agents.
func dispatchTasks(projectName string, tasks []taskDef, workDir string) (string, error) {
	teamName := generateTeamName()

	desc := fmt.Sprintf("Assistant: %d task(s) in %s", len(tasks), projectName)
	if _, err := agent.CreateTeam(teamName, desc, workDir); err != nil {
		return "", fmt.Errorf("create team: %w", err)
	}

	numWorkers := len(tasks)
	if numWorkers > 5 {
		numWorkers = 5
	}

	workers := make([]string, numWorkers)
	for i := range workers {
		workers[i] = fmt.Sprintf("worker-%d", i+1)
		if err := agent.AddMember(teamName, agent.TeamMember{
			Name:  workers[i],
			Role:  "Execute coding tasks",
			Model: "sonnet",
			Type:  "worker",
		}); err != nil {
			agent.DeleteTeam(teamName)
			return "", fmt.Errorf("add worker: %w", err)
		}
	}

	taskIDMap := make(map[int]int)
	for i, t := range tasks {
		var blockedBy []int
		for _, dep := range t.DependsOn {
			if id, ok := taskIDMap[dep]; ok {
				blockedBy = append(blockedBy, id)
			}
		}
		owner := workers[i%numWorkers]
		task, err := agent.CreateTask(teamName, t.Subject, t.Description, owner, blockedBy, agent.PriorityNormal, projectName, "")
		if err != nil {
			agent.DeleteTeam(teamName)
			return "", fmt.Errorf("create task: %w", err)
		}
		taskIDMap[i+1] = task.ID
	}

	if _, err := agent.StartAllAgents(teamName); err != nil {
		agent.DeleteTeam(teamName)
		return "", fmt.Errorf("start agents: %w", err)
	}

	return teamName, nil
}
