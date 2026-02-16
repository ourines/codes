package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// HookPayload is the JSON structure passed to hook scripts via stdin.
type HookPayload struct {
	Team      string `json:"team"`
	TaskID    int    `json:"taskId"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	Agent     string `json:"agent"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}

// HookRunner executes a shell hook script with a JSON payload on stdin.
type HookRunner struct {
	ScriptPath string
}

// NewHookRunner creates a HookRunner for the given script path.
func NewHookRunner(scriptPath string) *HookRunner {
	return &HookRunner{ScriptPath: scriptPath}
}

// Execute runs the hook script with a 30-second timeout.
// The JSON-encoded payload is passed via stdin.
func (h *HookRunner) Execute(payload HookPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, h.ScriptPath)

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("hook marshal payload: %w", err)
	}
	cmd.Stdin = strings.NewReader(string(data))

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("hook timed out after 30s: %s", h.ScriptPath)
	}
	if err != nil {
		return fmt.Errorf("hook execution failed: %w (output: %s)", err, string(output))
	}
	return nil
}
