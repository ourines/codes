package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// builtinWorkflows are the default workflow templates.
var builtinWorkflows = []Workflow{
	{
		Name:        "code-review",
		Description: "Review all staged changes for issues and improvements",
		BuiltIn:     true,
		Steps: []Step{
			{
				Name:   "Review staged changes",
				Prompt: "Run `git diff --cached` and review the staged changes. Look for bugs, security issues, code style problems, and suggest improvements. Be specific about file names and line numbers.",
			},
		},
	},
	{
		Name:        "write-tests",
		Description: "Generate tests for recently modified files",
		BuiltIn:     true,
		Steps: []Step{
			{
				Name:   "Identify modified files",
				Prompt: "Run `git diff --name-only HEAD~1` to find recently modified source files. List them and identify which ones lack test coverage.",
			},
			{
				Name:   "Write tests",
				Prompt: "Based on the modified files identified in the previous step, write comprehensive tests for any files lacking test coverage. Follow the existing test conventions in the project.",
			},
		},
	},
	{
		Name:        "pre-pr-check",
		Description: "Full pre-PR pipeline: review, test, docs",
		BuiltIn:     true,
		Steps: []Step{
			{
				Name:   "Code review",
				Prompt: "Run `git diff --cached` (or `git diff main...HEAD` if nothing staged) and review all changes. Report any bugs, security issues, or code quality concerns.",
			},
			{
				Name:   "Verify tests",
				Prompt: "Run the project's test suite. If tests fail, report the failures. If modified files lack tests, write them.",
			},
			{
				Name:            "Update documentation",
				Prompt:          "Check if any changed code requires documentation updates (README, inline comments, API docs). Make necessary updates.",
				WaitForApproval: true,
			},
		},
	},
}

// WorkflowDir returns the path to the workflows directory.
func WorkflowDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codes", "workflows")
}

// EnsureBuiltins writes built-in workflow templates to disk if they don't exist.
func EnsureBuiltins() error {
	dir := WorkflowDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, wf := range builtinWorkflows {
		path := filepath.Join(dir, wf.Name+".yml")
		if _, err := os.Stat(path); err == nil {
			continue // already exists
		}
		if err := saveWorkflowFile(path, &wf); err != nil {
			return err
		}
	}
	return nil
}

// ListWorkflows returns all workflows (built-in + user-defined).
func ListWorkflows() ([]Workflow, error) {
	if err := EnsureBuiltins(); err != nil {
		return nil, err
	}

	dir := WorkflowDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var workflows []Workflow
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		wf, err := loadWorkflowFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		// Mark built-ins
		for _, b := range builtinWorkflows {
			if b.Name == wf.Name {
				wf.BuiltIn = true
				break
			}
		}
		workflows = append(workflows, *wf)
	}
	return workflows, nil
}

// GetWorkflow returns a workflow by name.
func GetWorkflow(name string) (*Workflow, error) {
	if err := EnsureBuiltins(); err != nil {
		return nil, err
	}

	path := filepath.Join(WorkflowDir(), name+".yml")
	wf, err := loadWorkflowFile(path)
	if err != nil {
		// Try .yaml extension
		path = filepath.Join(WorkflowDir(), name+".yaml")
		wf, err = loadWorkflowFile(path)
		if err != nil {
			return nil, fmt.Errorf("workflow %q not found", name)
		}
	}

	for _, b := range builtinWorkflows {
		if b.Name == wf.Name {
			wf.BuiltIn = true
			break
		}
	}
	return wf, nil
}

// SaveWorkflow saves a workflow to YAML on disk.
func SaveWorkflow(wf *Workflow) error {
	if err := os.MkdirAll(WorkflowDir(), 0755); err != nil {
		return err
	}
	path := filepath.Join(WorkflowDir(), wf.Name+".yml")
	return saveWorkflowFile(path, wf)
}

// DeleteWorkflow removes a workflow file.
func DeleteWorkflow(name string) error {
	path := filepath.Join(WorkflowDir(), name+".yml")
	if err := os.Remove(path); err != nil {
		// Try .yaml
		path = filepath.Join(WorkflowDir(), name+".yaml")
		return os.Remove(path)
	}
	return nil
}

func loadWorkflowFile(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, err
	}
	return &wf, nil
}

func saveWorkflowFile(path string, wf *Workflow) error {
	data, err := yaml.Marshal(wf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
