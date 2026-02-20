package chatsession

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// spawnClaude starts a Claude CLI subprocess in stream-json mode.
// If resumeSessionID is non-empty, the session is resumed.
// Returns stdin writer, stdout reader, the command, and any error.
func spawnClaude(projectPath, model, resumeSessionID string) (io.WriteCloser, io.ReadCloser, *exec.Cmd, error) {
	args := []string{
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
	}

	if model != "" {
		args = append(args, "--model", model)
	}

	if resumeSessionID != "" {
		args = append(args, "--resume", resumeSessionID)
	}

	cmd := exec.Command("claude", args...)
	cmd.Dir = projectPath

	// Build clean environment, unsetting CLAUDE_CODE_ENTRYPOINT to avoid nested detection.
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "CLAUDE_CODE_ENTRYPOINT=") {
			filtered = append(filtered, e)
		}
	}
	cmd.Env = filtered

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	// Discard stderr to avoid blocking.
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, nil, nil, fmt.Errorf("start claude: %w", err)
	}

	return stdin, stdout, cmd, nil
}
