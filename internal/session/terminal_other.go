//go:build !darwin && !linux && !windows

package session

import (
	"fmt"

	"codes/internal/config"
)

func openInTerminal(sessionID, dir string, args []string, env map[string]string, terminal string) (int, error) {
	return 0, fmt.Errorf("terminal sessions not supported on this platform")
}

func focusTerminalWindow(terminal string) {}

func openRemoteInTerminal(sessionID string, host *config.RemoteHost, project string, terminal string) (int, error) {
	return 0, fmt.Errorf("terminal sessions not supported on this platform")
}
