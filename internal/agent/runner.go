package agent

import (
	"context"
	"time"
)

// RunClaude executes a Claude CLI subprocess and returns the parsed result.
// This function maintains backward compatibility by delegating to ClaudeAdapter.
//
// Deprecated: Use RunWithAdapter() for more flexibility. This function is kept
// for backward compatibility and will delegate to ClaudeAdapter.
func RunClaude(ctx context.Context, opts RunOptions) (*ClaudeResult, error) {
	adapter := &ClaudeAdapter{}

	cfg := RunConfig{
		Prompt:       opts.Prompt,
		WorkDir:      opts.WorkDir,
		Model:        opts.Model,
		SessionID:    opts.SessionID,
		Resume:       opts.Resume,
		Env:          opts.Env,
		SystemPrompt: opts.SystemPrompt,
		AllowedTools: opts.AllowedTools,
		MaxTurns:     opts.MaxTurns,
		PermMode:     opts.PermMode,
	}

	result, err := adapter.Run(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Convert RunResult to ClaudeResult for backward compatibility
	claudeResult := &ClaudeResult{
		Result:    result.Result,
		Error:     result.Error,
		SessionID: result.SessionID,
		IsError:   result.Error != "",
	}

	if result.Cost != nil {
		claudeResult.CostUSD = result.Cost.TotalCostUSD
	}

	if result.Duration > 0 {
		claudeResult.Duration = result.Duration.Seconds()
	}

	return claudeResult, nil
}

// RunWithAdapter executes a task using the specified CLI adapter.
// This is the preferred method for new code.
func RunWithAdapter(ctx context.Context, adapterName string, opts RunOptions) (*ClaudeResult, error) {
	// Get the adapter
	adapter, err := GetAdapter(adapterName)
	if err != nil {
		return nil, err
	}

	cfg := RunConfig{
		Prompt:       opts.Prompt,
		WorkDir:      opts.WorkDir,
		Model:        opts.Model,
		SessionID:    opts.SessionID,
		Resume:       opts.Resume,
		Env:          opts.Env,
		SystemPrompt: opts.SystemPrompt,
		AllowedTools: opts.AllowedTools,
		MaxTurns:     opts.MaxTurns,
		PermMode:     opts.PermMode,
		Timeout:      30 * time.Minute, // Default timeout
	}

	result, err := adapter.Run(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Convert RunResult to ClaudeResult
	claudeResult := &ClaudeResult{
		Result:    result.Result,
		Error:     result.Error,
		SessionID: result.SessionID,
		IsError:   result.Error != "",
	}

	if result.Cost != nil {
		claudeResult.CostUSD = result.Cost.TotalCostUSD
	}

	if result.Duration > 0 {
		claudeResult.Duration = result.Duration.Seconds()
	}

	return claudeResult, nil
}

