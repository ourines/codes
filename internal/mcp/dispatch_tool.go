package mcpserver

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"codes/internal/dispatch"
)

// -- dispatch --

type dispatchInput struct {
	Text        string `json:"text" jsonschema:"Natural language task description"`
	Project     string `json:"project,omitempty" jsonschema:"Target project name (auto-detected if omitted)"`
	Channel     string `json:"channel,omitempty" jsonschema:"Source channel identifier (default: mcp)"`
	CallbackURL string `json:"callback_url,omitempty" jsonschema:"URL to POST when tasks complete"`
	Model       string `json:"model,omitempty" jsonschema:"Model for intent analysis (default: haiku)"`
}

type dispatchOutput struct {
	Team          string `json:"team,omitempty"`
	TasksCreated  int    `json:"tasks_created,omitempty"`
	AgentsStarted int    `json:"agents_started,omitempty"`
	Clarify       string `json:"clarify,omitempty"`
	Duration      string `json:"duration,omitempty"`
	Error         string `json:"error,omitempty"`
}

func dispatchHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input dispatchInput) (*mcpsdk.CallToolResult, dispatchOutput, error) {
	if input.Channel == "" {
		input.Channel = "mcp"
	}

	result, err := dispatch.Dispatch(ctx, dispatch.DispatchOptions{
		UserInput:   input.Text,
		Channel:     input.Channel,
		Project:     input.Project,
		CallbackURL: input.CallbackURL,
		Model:       input.Model,
	})
	if err != nil {
		return nil, dispatchOutput{Error: err.Error()}, nil
	}

	return nil, dispatchOutput{
		Team:          result.TeamName,
		TasksCreated:  result.TasksCreated,
		AgentsStarted: result.AgentsStarted,
		Clarify:       result.Clarify,
		Duration:      result.DurationStr,
		Error:         result.Error,
	}, nil
}

func registerDispatchTool(server *mcpsdk.Server) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "dispatch",
		Description: `Dispatch a natural language task request to an autonomous agent team.
Uses AI to analyze the request, identify the target project, break it into tasks,
assign workers, and start execution — all in one step.

Returns team name for monitoring with team_status/team_watch.
If the request is ambiguous, returns a 'clarify' message instead of creating tasks.`,
	}, dispatchHandler)
}
