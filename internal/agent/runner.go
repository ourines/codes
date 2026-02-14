package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunClaude executes a Claude CLI subprocess and returns the parsed result.
// It uses `claude -p "prompt" --output-format json` with optional flags.
func RunClaude(ctx context.Context, opts RunOptions) (*ClaudeResult, error) {
	args := buildClaudeArgs(opts)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = opts.WorkDir

	// Set up environment
	cmd.Env = os.Environ()
	for k, v := range opts.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set platform-specific process attributes
	setSysProcAttr(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Parse JSON output
	result := &ClaudeResult{}
	outBytes := stdout.Bytes()

	if len(outBytes) > 0 {
		if err := parseClaudeOutput(outBytes, result); err != nil {
			// If JSON parsing fails, use raw output
			result.Result = string(outBytes)
		}
	}

	if err != nil {
		result.IsError = true
		if result.Error == "" {
			errText := stderr.String()
			if errText == "" {
				errText = err.Error()
			}
			result.Error = errText
		}
	}

	return result, nil
}

// buildClaudeArgs constructs the command-line arguments for claude.
func buildClaudeArgs(opts RunOptions) []string {
	var args []string

	// Print mode with prompt
	args = append(args, "-p", opts.Prompt)

	// JSON output
	args = append(args, "--output-format", "json")

	// Session continuity
	if opts.SessionID != "" {
		if opts.Resume {
			args = append(args, "--resume", "--session-id", opts.SessionID)
		} else {
			args = append(args, "--session-id", opts.SessionID)
		}
	}

	// Model selection
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	// System prompt
	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}

	// Allowed tools
	for _, tool := range opts.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	// Max turns
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}

	// Permission mode
	if opts.PermMode != "" {
		args = append(args, "--"+opts.PermMode)
	}

	return args
}

// claudeJSONOutput represents the JSON output from claude CLI.
type claudeJSONOutput struct {
	Type      string  `json:"type"`
	Result    string  `json:"result"`
	Cost      float64 `json:"cost_usd"`
	Duration  float64 `json:"duration_secs"`
	SessionID string  `json:"session_id"`
	IsError   bool    `json:"is_error"`
}

// parseClaudeOutput parses the JSON output from claude CLI.
// The output may be a single JSON object or newline-delimited JSON.
func parseClaudeOutput(data []byte, result *ClaudeResult) error {
	// Try parsing as single JSON object first
	var out claudeJSONOutput
	if err := json.Unmarshal(data, &out); err == nil {
		result.Result = out.Result
		result.SessionID = out.SessionID
		result.CostUSD = out.Cost
		result.Duration = out.Duration
		result.IsError = out.IsError
		return nil
	}

	// Try newline-delimited JSON, take the last "result" type message
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var out claudeJSONOutput
		if err := json.Unmarshal([]byte(line), &out); err != nil {
			continue
		}
		if out.Type == "result" || out.Result != "" {
			result.Result = out.Result
			result.SessionID = out.SessionID
			result.CostUSD = out.Cost
			result.Duration = out.Duration
			result.IsError = out.IsError
			return nil
		}
	}

	return fmt.Errorf("no result found in claude output")
}
