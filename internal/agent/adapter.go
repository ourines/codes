package agent

import (
	"context"
	"time"
)

// CLIAdapter defines the interface for AI CLI tool adapters.
// Each adapter wraps a specific CLI tool (claude, aichat, mods, etc.)
// and provides a unified interface for task execution.
type CLIAdapter interface {
	// Name returns the adapter identifier (e.g., "claude", "aichat")
	Name() string

	// Available checks if the CLI tool is installed and executable
	Available() bool

	// Capabilities returns the feature set supported by this adapter
	Capabilities() AdapterCapabilities

	// Run executes a task using the CLI tool
	Run(ctx context.Context, cfg RunConfig) (*RunResult, error)
}

// AdapterCapabilities declares what features an adapter supports.
type AdapterCapabilities struct {
	SessionPersistence bool // Session management with resumable contexts
	JSONOutput         bool // Structured JSON output
	ModelSelection     bool // Model selection (e.g., opus vs sonnet)
	CostTracking       bool // Token cost reporting
}

// RunConfig specifies the parameters for a CLI adapter execution.
// This is an adapter-agnostic configuration that maps to adapter-specific flags.
type RunConfig struct {
	Prompt    string            // Task prompt
	WorkDir   string            // Working directory
	Model     string            // Model name (adapter-specific)
	SessionID string            // Session ID for continuity
	Resume    bool              // Resume from existing session
	Timeout   time.Duration     // Execution timeout
	Env       map[string]string // Environment variables

	// Claude-specific (optional for other adapters)
	SystemPrompt string   // System prompt
	AllowedTools []string // Allowed tools
	MaxTurns     int      // Max agentic turns
	PermMode     string   // Permission mode
}

// RunResult holds the output from a CLI adapter execution.
type RunResult struct {
	Result    string  // Main response content
	Error     string  // Error message (if any)
	SessionID string  // New/updated session ID
	Cost      *CostInfo // Cost information (if available)
	Duration  time.Duration
}

// CostInfo holds token usage and cost details.
type CostInfo struct {
	InputTokens      int     `json:"inputTokens,omitempty"`
	OutputTokens     int     `json:"outputTokens,omitempty"`
	CacheReadTokens  int     `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens int     `json:"cacheWriteTokens,omitempty"`
	TotalCostUSD     float64 `json:"totalCostUSD,omitempty"`
}
