package mcpserver

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"codes/internal/assistant"
)

// -- dispatch --
// Routes natural language requests through the personal assistant,
// which uses tools (run_tasks, list_projects, etc.) to act on the request.

type dispatchInput struct {
	Text      string `json:"text" jsonschema:"Natural language task description or request"`
	SessionID string `json:"session_id,omitempty" jsonschema:"Conversation session ID (default: dispatch)"`
}

type dispatchOutput struct {
	Reply string `json:"reply"`
}

func dispatchHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input dispatchInput) (*mcpsdk.CallToolResult, dispatchOutput, error) {
	if input.Text == "" {
		return nil, dispatchOutput{}, fmt.Errorf("field 'text' is required")
	}
	sessionID := input.SessionID
	if sessionID == "" {
		sessionID = "dispatch"
	}

	result, err := assistant.Run(ctx, assistant.RunOptions{
		SessionID: sessionID,
		Message:   input.Text,
	})
	if err != nil {
		return nil, dispatchOutput{}, fmt.Errorf("assistant error: %w", err)
	}

	return nil, dispatchOutput{Reply: result.Reply}, nil
}

func registerDispatchTool(server *mcpsdk.Server) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "dispatch",
		Description: `Send a natural language request to the personal assistant.
The assistant understands intent, manages agent teams, runs tasks, and remembers context.

Examples:
  "Give myproject a code review"         → creates team, runs agents
  "What's the status of team foo?"       → calls get_team_status
  "Stop all agents in team bar"          → sends stop signals
  "Remind me to deploy at 5pm"           → sets a reminder`,
	}, dispatchHandler)
}
