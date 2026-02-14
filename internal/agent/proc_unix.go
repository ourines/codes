//go:build !windows

package agent

import (
	"os"
	"os/exec"
	"syscall"
)

// setSysProcAttr configures platform-specific process attributes.
// On Unix, we use Setpgid to create a new process group so the child
// can be killed independently of the parent.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// isProcessAlive checks if a process with the given PID is still running.
func isProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil
}
