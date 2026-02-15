package workflow

import (
	"fmt"
	"time"

	"codes/internal/agent"
)

// RunWorkflowOptions configures workflow execution.
type RunWorkflowOptions struct {
	WorkDir string
	Model   string
	Project string
}

// RunWorkflow launches a workflow as an agent team (non-blocking).
// It creates a team, adds agents, creates tasks, and starts all agents.
func RunWorkflow(wf *Workflow, opts RunWorkflowOptions) (*WorkflowRunResult, error) {
	if len(wf.Agents) == 0 {
		return nil, fmt.Errorf("workflow %q has no agents defined", wf.Name)
	}
	if len(wf.Tasks) == 0 {
		return nil, fmt.Errorf("workflow %q has no tasks defined", wf.Name)
	}

	// Validate task.Assign references existing agents
	agentNames := make(map[string]bool)
	for _, a := range wf.Agents {
		agentNames[a.Name] = true
	}
	for i, t := range wf.Tasks {
		if t.Assign != "" && !agentNames[t.Assign] {
			return nil, fmt.Errorf("task %d (%q) assigns to unknown agent %q", i+1, t.Subject, t.Assign)
		}
		// Validate blockedBy references are in range
		for _, dep := range t.BlockedBy {
			if dep < 1 || dep > len(wf.Tasks) {
				return nil, fmt.Errorf("task %d (%q) has invalid blockedBy index %d (must be 1-%d)", i+1, t.Subject, dep, len(wf.Tasks))
			}
			if dep == i+1 {
				return nil, fmt.Errorf("task %d (%q) cannot block itself", i+1, t.Subject)
			}
		}
	}

	// Generate unique team name
	teamName := fmt.Sprintf("wf-%s-%d", wf.Name, time.Now().Unix())

	// Create team
	_, err := agent.CreateTeam(teamName, fmt.Sprintf("Workflow: %s", wf.Description), opts.WorkDir)
	if err != nil {
		// Handle second-level collision
		teamName = fmt.Sprintf("wf-%s-%d", wf.Name, time.Now().UnixMilli())
		_, err = agent.CreateTeam(teamName, fmt.Sprintf("Workflow: %s", wf.Description), opts.WorkDir)
		if err != nil {
			return nil, fmt.Errorf("create team: %w", err)
		}
	}

	// Add agents
	for _, a := range wf.Agents {
		model := a.Model
		if opts.Model != "" {
			model = opts.Model
		}
		member := agent.TeamMember{
			Name:  a.Name,
			Role:  a.Role,
			Model: model,
			Type:  "worker",
		}
		if err := agent.AddMember(teamName, member); err != nil {
			agent.DeleteTeam(teamName)
			return nil, fmt.Errorf("add agent %q: %w", a.Name, err)
		}
	}

	// Create tasks, tracking the mapping from 1-based index to actual task ID
	taskIDMap := make(map[int]int) // 1-based workflow index → actual task ID
	for i, t := range wf.Tasks {
		// Map blockedBy from 1-based workflow index to actual task IDs
		var blockedBy []int
		for _, dep := range t.BlockedBy {
			if actualID, ok := taskIDMap[dep]; ok {
				blockedBy = append(blockedBy, actualID)
			}
		}

		priority := agent.PriorityNormal
		if t.Priority != "" {
			priority = agent.TaskPriority(t.Priority)
		}

		task, err := agent.CreateTask(
			teamName,
			t.Subject,
			t.Prompt,
			t.Assign,
			blockedBy,
			priority,
			opts.Project,
			opts.WorkDir,
		)
		if err != nil {
			agent.DeleteTeam(teamName)
			return nil, fmt.Errorf("create task %q: %w", t.Subject, err)
		}
		taskIDMap[i+1] = task.ID
	}

	// Start all agents
	_, err = agent.StartAllAgents(teamName)
	if err != nil {
		agent.DeleteTeam(teamName)
		return nil, fmt.Errorf("start agents: %w", err)
	}

	return &WorkflowRunResult{
		TeamName: teamName,
		Agents:   len(wf.Agents),
		Tasks:    len(wf.Tasks),
	}, nil
}
