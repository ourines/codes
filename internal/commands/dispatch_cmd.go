package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codes/internal/dispatch"
	"codes/internal/output"
	"codes/internal/ui"
)

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

// RunDispatch executes the dispatch command: analyzes the text with AI, creates a team and tasks.
func RunDispatch(text, project, model, callbackURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := dispatch.Dispatch(ctx, dispatch.DispatchOptions{
		UserInput:   text,
		Channel:     "cli",
		Project:     project,
		Model:       model,
		CallbackURL: callbackURL,
	})
	if err != nil {
		ui.ShowError("Dispatch failed", err)
		return
	}

	if output.JSONMode {
		printJSON(result)
		return
	}

	if result.Clarify != "" {
		fmt.Printf("Clarification needed: %s\n", result.Clarify)
		return
	}
	if result.Error != "" {
		ui.ShowError("Dispatch error", fmt.Errorf("%s", result.Error))
		return
	}

	fmt.Printf("Team:    %s\n", result.TeamName)
	fmt.Printf("Tasks:   %d created\n", result.TasksCreated)
	fmt.Printf("Agents:  %d started\n", result.AgentsStarted)
	fmt.Printf("Time:    %s\n", result.DurationStr)
}
