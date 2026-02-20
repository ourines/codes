package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codes/internal/assistant"
	"codes/internal/output"
	"codes/internal/ui"
)

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

// RunDispatch sends a natural language request to the personal assistant.
func RunDispatch(text, project, model, callbackURL string) {
	// Prepend project hint if specified
	msg := text
	if project != "" {
		msg = fmt.Sprintf("[project: %s] %s", project, text)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	result, err := assistant.Run(ctx, assistant.RunOptions{
		SessionID: "dispatch",
		Message:   msg,
	})
	if err != nil {
		ui.ShowError("Dispatch failed", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]string{"reply": result.Reply})
		return
	}

	fmt.Println(result.Reply)
}
