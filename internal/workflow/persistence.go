package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GetRunsDir returns the directory where workflow runs are stored.
func GetRunsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".codes", "workflow-runs")
	}
	return filepath.Join(home, ".codes", "workflow-runs")
}

// SaveRun persists a workflow run to disk.
func SaveRun(run *WorkflowRun) error {
	dir := GetRunsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create runs directory: %w", err)
	}

	// Generate ID if not set
	if run.ID == "" {
		run.ID = fmt.Sprintf("%s-%s", run.Workflow.Name, time.Now().Format("20060102-150405"))
	}

	// Update timestamps
	if run.StartedAt == "" {
		run.StartedAt = time.Now().Format(time.RFC3339)
	}
	if run.Status == "completed" || run.Status == "failed" || run.Status == "aborted" {
		run.CompletedAt = time.Now().Format(time.RFC3339)
	}

	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}

	filename := filepath.Join(dir, run.ID+".json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("write run file: %w", err)
	}

	return nil
}

// LoadRun loads a workflow run from disk by ID.
func LoadRun(id string) (*WorkflowRun, error) {
	dir := GetRunsDir()
	filename := filepath.Join(dir, id+".json")

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read run file: %w", err)
	}

	var run WorkflowRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("unmarshal run: %w", err)
	}

	return &run, nil
}

// ListRuns returns all workflow runs sorted by start time (newest first).
func ListRuns() ([]*WorkflowRun, error) {
	dir := GetRunsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*WorkflowRun{}, nil
		}
		return nil, fmt.Errorf("read runs directory: %w", err)
	}

	var runs []*WorkflowRun
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		run, err := LoadRun(id)
		if err != nil {
			continue // Skip corrupted files
		}
		runs = append(runs, run)
	}

	return runs, nil
}

// DeleteRun removes a workflow run from disk.
func DeleteRun(id string) error {
	dir := GetRunsDir()
	filename := filepath.Join(dir, id+".json")
	return os.Remove(filename)
}
