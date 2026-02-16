package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ClaudeAdapter implements CLIAdapter for the Claude CLI tool.
type ClaudeAdapter struct{}

func init() {
	RegisterAdapter("claude", &ClaudeAdapter{})
}

// Name returns the adapter identifier.
func (a *ClaudeAdapter) Name() string {
	return "claude"
}

// Available checks if the claude CLI is installed and executable.
func (a *ClaudeAdapter) Available() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

// Capabilities returns the full feature set of Claude CLI.
func (a *ClaudeAdapter) Capabilities() AdapterCapabilities {
	return AdapterCapabilities{
		SessionPersistence: true,
		JSONOutput:         true,
		ModelSelection:     true,
		CostTracking:       true,
	}
}

// Run executes a Claude CLI subprocess with the given configuration.
// This implementation is extracted from the original RunClaude() function
// to maintain full backward compatibility.
func (a *ClaudeAdapter) Run(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	// Validate permission mode
	if cfg.PermMode != "" {
		validPermModes := map[string]bool{"dangerously-skip-permissions": true}
		if !validPermModes[cfg.PermMode] {
			return nil, fmt.Errorf("invalid permission mode: %q", cfg.PermMode)
		}
	}

	args := a.buildArgs(cfg)
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = cfg.WorkDir

	// Set up environment
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set platform-specific process attributes
	setSysProcAttr(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Parse JSON output
	result := &RunResult{}
	outBytes := stdout.Bytes()

	if len(outBytes) > 0 {
		if parseErr := a.parseOutput(outBytes, result); parseErr != nil {
			// If JSON parsing fails, use raw output
			result.Result = string(outBytes)
		}
	}

	if err != nil {
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

// buildArgs constructs the command-line arguments for claude.
// This is extracted from the original buildClaudeArgs() function.
func (a *ClaudeAdapter) buildArgs(cfg RunConfig) []string {
	var args []string

	// Print mode with prompt
	args = append(args, "-p", cfg.Prompt)

	// JSON output
	args = append(args, "--output-format", "json")

	// Session continuity: --session-id with --resume requires --fork-session
	if cfg.SessionID != "" && cfg.Resume {
		args = append(args, "--resume", "--session-id", cfg.SessionID, "--fork-session")
	}

	// Model selection
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	// System prompt
	if cfg.SystemPrompt != "" {
		args = append(args, "--system-prompt", cfg.SystemPrompt)
	}

	// Allowed tools
	for _, tool := range cfg.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	// Max turns
	if cfg.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", cfg.MaxTurns))
	}

	// Permission mode
	if cfg.PermMode != "" {
		args = append(args, "--"+cfg.PermMode)
	}

	return args
}

// claudeJSONOutput represents the JSON output from claude CLI.
// This matches the format returned by `claude --output-format json`.
type claudeJSONOutput struct {
	Type      string  `json:"type"`
	Result    string  `json:"result"`
	Cost      float64 `json:"cost_usd"`
	Duration  float64 `json:"duration_secs"`
	SessionID string  `json:"session_id"`
	IsError   bool    `json:"is_error"`
}

// parseOutput parses the JSON output from claude CLI.
// The output may be a single JSON object or newline-delimited JSON.
// This is extracted from the original parseClaudeOutput() function.
func (a *ClaudeAdapter) parseOutput(data []byte, result *RunResult) error {
	// Try parsing as single JSON object first
	var out claudeJSONOutput
	if err := json.Unmarshal(data, &out); err == nil {
		result.Result = out.Result
		result.SessionID = out.SessionID
		if out.Cost > 0 {
			result.Cost = &CostInfo{TotalCostUSD: out.Cost}
		}
		if out.Duration > 0 {
			result.Duration = time.Duration(out.Duration * float64(time.Second))
		}
		if out.IsError && result.Error == "" {
			result.Error = out.Result
			if result.Error == "" {
				result.Error = "claude reported is_error with no details"
			}
		}
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
			if out.Cost > 0 {
				result.Cost = &CostInfo{TotalCostUSD: out.Cost}
			}
			if out.Duration > 0 {
				result.Duration = time.Duration(out.Duration * float64(time.Second))
			}
			if out.IsError && result.Error == "" {
				result.Error = out.Result
				if result.Error == "" {
					result.Error = "claude reported is_error with no details"
				}
			}
			return nil
		}
	}

	return fmt.Errorf("no result found in claude output")
}
