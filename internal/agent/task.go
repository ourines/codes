package agent

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// CreateTask creates a new task in a team.
func CreateTask(teamName, subject, description, owner string, blockedBy []int, priority TaskPriority) (*Task, error) {
	if err := ensureDir(tasksDir(teamName)); err != nil {
		return nil, err
	}

	id, err := nextTaskID(teamName)
	if err != nil {
		return nil, fmt.Errorf("next task ID: %w", err)
	}

	now := time.Now()
	status := TaskPending
	if owner != "" {
		status = TaskAssigned
	}

	if priority == "" {
		priority = PriorityNormal
	}

	task := &Task{
		ID:          id,
		Subject:     subject,
		Description: description,
		Status:      status,
		Priority:    priority,
		Owner:       owner,
		BlockedBy:   blockedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := writeJSON(taskPath(teamName, id), task); err != nil {
		return nil, fmt.Errorf("write task: %w", err)
	}

	return task, nil
}

// GetTask loads a single task by ID.
func GetTask(teamName string, taskID int) (*Task, error) {
	var task Task
	path := taskPath(teamName, taskID)
	if err := readJSON(path, &task); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task %d not found in team %q", taskID, teamName)
		}
		return nil, err
	}
	return &task, nil
}

// ListTasks returns all tasks for a team, optionally filtered.
func ListTasks(teamName string, statusFilter TaskStatus, ownerFilter string) ([]*Task, error) {
	dir := tasksDir(teamName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var tasks []*Task
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") || strings.Contains(name, ".json.") {
			continue
		}
		var id int
		if _, err := fmt.Sscanf(name, "%d.json", &id); err != nil {
			continue
		}

		var task Task
		if err := readJSON(taskPath(teamName, id), &task); err != nil {
			continue
		}

		if statusFilter != "" && task.Status != statusFilter {
			continue
		}
		if ownerFilter != "" && task.Owner != ownerFilter {
			continue
		}

		tasks = append(tasks, &task)
	}

	// Sort by priority (high > normal > low), then by ID (ascending)
	sort.Slice(tasks, func(i, j int) bool {
		pi, pj := priorityRank(tasks[i].Priority), priorityRank(tasks[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return tasks[i].ID < tasks[j].ID
	})

	return tasks, nil
}

// priorityRank returns a sort rank for a priority level (lower = higher priority).
func priorityRank(p TaskPriority) int {
	switch p {
	case PriorityHigh:
		return 0
	case PriorityLow:
		return 2
	default:
		return 1 // normal or empty
	}
}

// UpdateTask modifies a task with file locking for safe concurrent access.
func UpdateTask(teamName string, taskID int, updateFn func(*Task) error) (*Task, error) {
	lockPath := taskLockPath(teamName, taskID)
	if err := ensureDir(tasksDir(teamName)); err != nil {
		return nil, err
	}

	fl := NewFileLock(lockPath)
	if err := fl.Lock(); err != nil {
		return nil, fmt.Errorf("lock task %d: %w", taskID, err)
	}
	defer fl.Unlock()

	task, err := GetTask(teamName, taskID)
	if err != nil {
		return nil, err
	}

	if err := updateFn(task); err != nil {
		return nil, err
	}

	task.UpdatedAt = time.Now()

	if err := writeJSON(taskPath(teamName, taskID), task); err != nil {
		return nil, fmt.Errorf("write task: %w", err)
	}

	return task, nil
}

// AssignTask assigns a task to an agent.
func AssignTask(teamName string, taskID int, owner string) (*Task, error) {
	return UpdateTask(teamName, taskID, func(t *Task) error {
		if t.Status != TaskPending {
			return fmt.Errorf("cannot assign task %d: status is %s (must be pending)", taskID, t.Status)
		}
		t.Status = TaskAssigned
		t.Owner = owner
		return nil
	})
}

// CompleteTask marks a task as completed with a result.
func CompleteTask(teamName string, taskID int, result string) (*Task, error) {
	return UpdateTask(teamName, taskID, func(t *Task) error {
		if t.Status != TaskRunning && t.Status != TaskAssigned {
			return fmt.Errorf("cannot complete task %d: status is %s", taskID, t.Status)
		}
		t.Status = TaskCompleted
		t.Result = result
		now := time.Now()
		t.CompletedAt = &now
		return nil
	})
}

// FailTask marks a task as failed with an error message.
func FailTask(teamName string, taskID int, errMsg string) (*Task, error) {
	return UpdateTask(teamName, taskID, func(t *Task) error {
		if t.Status != TaskRunning && t.Status != TaskAssigned {
			return fmt.Errorf("cannot fail task %d: status is %s", taskID, t.Status)
		}
		t.Status = TaskFailed
		t.Error = errMsg
		now := time.Now()
		t.CompletedAt = &now
		return nil
	})
}

// CancelTask cancels a task.
func CancelTask(teamName string, taskID int) (*Task, error) {
	return UpdateTask(teamName, taskID, func(t *Task) error {
		if t.Status == TaskCompleted || t.Status == TaskCancelled {
			return fmt.Errorf("cannot cancel task %d: status is %s", taskID, t.Status)
		}
		t.Status = TaskCancelled
		now := time.Now()
		t.CompletedAt = &now
		return nil
	})
}

// IsTaskBlocked checks if a task's dependencies are all completed.
func IsTaskBlocked(teamName string, task *Task) (bool, error) {
	if len(task.BlockedBy) == 0 {
		return false, nil
	}

	for _, depID := range task.BlockedBy {
		dep, err := GetTask(teamName, depID)
		if err != nil {
			return true, err
		}
		if dep.Status != TaskCompleted {
			return true, nil
		}
	}
	return false, nil
}
