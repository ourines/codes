package commands

import (
	"fmt"
	"strconv"

	"codes/internal/agent"
	"codes/internal/output"
	"codes/internal/ui"
)

// RunTaskSimpleAdd creates a task with minimal arguments.
func RunTaskSimpleAdd(teamName, description, assign string) {
	task, err := agent.CreateTask(teamName, description, "", assign, nil, agent.PriorityNormal, "", "")
	if err != nil {
		ui.ShowError("Failed to create task", err)
		return
	}

	if output.JSONMode {
		printJSON(task)
		return
	}
	fmt.Printf("Task #%d created", task.ID)
	if task.Owner != "" {
		fmt.Printf(" → %s", task.Owner)
	}
	fmt.Println()
}

// RunTaskSimpleList lists tasks across one or all teams.
func RunTaskSimpleList(teamName string) {
	var teams []string
	if teamName != "" {
		teams = []string{teamName}
	} else {
		var err error
		teams, err = agent.ListTeams()
		if err != nil {
			ui.ShowError("Failed to list teams", err)
			return
		}
	}

	if output.JSONMode {
		allTasks := make(map[string][]*agent.Task)
		for _, t := range teams {
			tasks, err := agent.ListTasks(t, "", "")
			if err != nil {
				continue
			}
			allTasks[t] = tasks
		}
		printJSON(allTasks)
		return
	}

	for _, t := range teams {
		tasks, err := agent.ListTasks(t, "", "")
		if err != nil {
			fmt.Printf("  %s: error: %v\n", t, err)
			continue
		}
		if len(tasks) == 0 {
			continue
		}

		fmt.Printf("[%s]\n", t)
		for _, task := range tasks {
			icon := statusIcon(task.Status)
			owner := ""
			if task.Owner != "" {
				owner = fmt.Sprintf(" → %s", task.Owner)
			}
			fmt.Printf("  %s #%-4d %s%s\n", icon, task.ID, task.Subject, owner)
		}
		fmt.Println()
	}
}

// RunTaskSimpleResult shows the result of a specific task.
func RunTaskSimpleResult(teamName, taskIDStr string) {
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

	fmt.Printf("#%d [%s] %s\n", task.ID, task.Status, task.Subject)
	if task.Owner != "" {
		fmt.Printf("  Owner: %s\n", task.Owner)
	}
	if task.Result != "" {
		fmt.Printf("\n--- Result ---\n%s\n", task.Result)
	}
	if task.Error != "" {
		fmt.Printf("\n--- Error ---\n%s\n", task.Error)
	}
}

// statusIcon returns a compact status indicator.
func statusIcon(s agent.TaskStatus) string {
	switch s {
	case agent.TaskPending:
		return "○"
	case agent.TaskAssigned:
		return "◆"
	case agent.TaskRunning:
		return "▶"
	case agent.TaskCompleted:
		return "✓"
	case agent.TaskFailed:
		return "✗"
	case agent.TaskCancelled:
		return "—"
	default:
		return "?"
	}
}
