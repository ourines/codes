package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	"codes/internal/assistant"
	"codes/internal/output"
	"codes/internal/ui"
)

// RunAssistantOnce sends a single message and prints the reply.
func RunAssistantOnce(message, sessionID, model string) error {
	ui.ShowInfo("Thinking...")

	result, err := assistant.Run(context.Background(), assistant.RunOptions{
		SessionID: sessionID,
		Message:   message,
		Model:     anthropic.Model(model),
	})
	if err != nil {
		ui.ShowError("Assistant error", err)
		return err
	}

	if output.JSONMode {
		output.Print(map[string]string{
			"session": sessionID,
			"reply":   result.Reply,
		}, nil)
		return nil
	}

	fmt.Println(result.Reply)
	return nil
}

// RunAssistantREPL starts an interactive conversation loop.
func RunAssistantREPL(sessionID, model string) error {
	fmt.Printf("Assistant (session: %s) — type 'exit' or Ctrl-C to quit\n\n", sessionID)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}

		result, err := assistant.Run(context.Background(), assistant.RunOptions{
			SessionID: sessionID,
			Message:   line,
			Model:     anthropic.Model(model),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}

		fmt.Printf("\n%s\n\n", result.Reply)
	}
	return nil
}

// RunAssistantClear deletes the session history.
func RunAssistantClear(sessionID string) error {
	if err := assistant.ClearSession(sessionID); err != nil {
		ui.ShowError("Failed to clear session", err)
		return err
	}
	if output.JSONMode {
		output.Print(map[string]string{"session": sessionID, "status": "cleared"}, nil)
		return nil
	}
	fmt.Printf("Session %q cleared.\n", sessionID)
	return nil
}
