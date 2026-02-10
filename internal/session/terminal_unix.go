//go:build !windows

package session

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// isProcessAlive checks if a process with the given PID is still running.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// killProcess sends SIGTERM to the process, then SIGKILL if needed.
func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

// buildScript creates a shell script that launches Claude Code in the given directory.
// The script writes its PID to a file and cleans up on exit.
func buildScript(name, dir string, args []string, env map[string]string) (script string, scriptPath string) {
	pidFile := pidFilePath(name)
	scriptPath = fmt.Sprintf("%s/codes-%s.sh", os.TempDir(), name)

	var b strings.Builder
	b.WriteString("#!/bin/bash\n")
	b.WriteString(fmt.Sprintf("# codes session: %s\n\n", name))

	// Cleanup on exit (removes PID file and this script)
	b.WriteString(fmt.Sprintf("cleanup() { rm -f '%s' '%s'; }\n", pidFile, scriptPath))
	b.WriteString("trap cleanup EXIT\n\n")

	// Write PID file
	b.WriteString(fmt.Sprintf("echo $$ > '%s'\n\n", pidFile))

	// Set environment variables
	for k, v := range env {
		escaped := strings.ReplaceAll(v, "'", "'\\''")
		b.WriteString(fmt.Sprintf("export %s='%s'\n", k, escaped))
	}
	if len(env) > 0 {
		b.WriteString("\n")
	}

	// Change to project directory
	escaped := strings.ReplaceAll(dir, "'", "'\\''")
	b.WriteString(fmt.Sprintf("cd '%s'\n\n", escaped))

	// Set window title
	b.WriteString(fmt.Sprintf("echo -ne '\\033]0;codes: %s\\007'\n\n", name))

	// Run claude
	if len(args) > 0 {
		quotedArgs := make([]string, len(args))
		for i, a := range args {
			quotedArgs[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(a, "'", "'\\''"))
		}
		b.WriteString(fmt.Sprintf("claude %s\n", strings.Join(quotedArgs, " ")))
	} else {
		b.WriteString("claude\n")
	}

	return b.String(), scriptPath
}
