//go:build windows

package session

import "fmt"

func openInTerminal(sessionID, dir string, args []string, env map[string]string, terminal string) (int, error) {
	return 0, fmt.Errorf("terminal sessions are not yet supported on Windows")
}

func focusTerminalWindow(terminal string) {}

func isProcessAlive(pid int) bool { return false }

func killProcess(pid int) error { return nil }
