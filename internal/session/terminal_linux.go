//go:build linux || freebsd || openbsd

package session

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"codes/internal/config"
)

// terminalEmulators lists common Linux terminal emulators and their launch arguments.
var terminalEmulators = []struct {
	name string
	args func(script string) []string
}{
	{"x-terminal-emulator", func(s string) []string { return []string{"-e", "bash", s} }},
	{"gnome-terminal", func(s string) []string { return []string{"--", "bash", s} }},
	{"konsole", func(s string) []string { return []string{"-e", "bash", s} }},
	{"xfce4-terminal", func(s string) []string { return []string{"-e", "bash " + s} }},
	{"alacritty", func(s string) []string { return []string{"-e", "bash", s} }},
	{"kitty", func(s string) []string { return []string{"bash", s} }},
	{"xterm", func(s string) []string { return []string{"-e", "bash", s} }},
}

// openInTerminal opens a new terminal window running Claude Code.
// The terminal parameter can specify a custom terminal emulator command.
func openInTerminal(sessionID, dir string, args []string, env map[string]string, terminal string) (int, error) {
	script, scriptPath := buildScript(sessionID, dir, args, env)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return 0, err
	}

	pidFile := pidFilePath(sessionID)
	os.Remove(pidFile)

	var started bool

	// If a specific terminal is configured, try it first
	if terminal != "" {
		cmd := exec.Command(terminal, "-e", "bash", scriptPath)
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
			started = true
		}
	}

	// Auto-detect terminal emulator
	if !started {
		for _, te := range terminalEmulators {
			if _, err := exec.LookPath(te.name); err != nil {
				continue
			}
			cmd := exec.Command(te.name, te.args(scriptPath)...)
			if err := cmd.Start(); err != nil {
				continue
			}
			go cmd.Wait()
			started = true
			break
		}
	}

	if !started {
		os.Remove(scriptPath)
		return 0, fmt.Errorf("no terminal emulator found")
	}

	// Wait for PID file to appear (up to 5 seconds)
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		data, err := os.ReadFile(pidFile)
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil && pid > 0 {
				return pid, nil
			}
		}
	}

	return 0, fmt.Errorf("timed out waiting for session PID")
}

// focusTerminalWindow is a no-op on Linux (window focusing is WM-dependent).
func focusTerminalWindow(terminal string) {}

// openRemoteInTerminal opens a new terminal window with an SSH session to a remote host.
func openRemoteInTerminal(sessionID string, host *config.RemoteHost, project string, terminal string) (int, error) {
	script, scriptPath := buildRemoteScript(sessionID, host, project)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return 0, err
	}

	pidFile := pidFilePath(sessionID)
	os.Remove(pidFile)

	var started bool

	if terminal != "" {
		cmd := exec.Command(terminal, "-e", "bash", scriptPath)
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
			started = true
		}
	}

	if !started {
		for _, te := range terminalEmulators {
			if _, err := exec.LookPath(te.name); err != nil {
				continue
			}
			cmd := exec.Command(te.name, te.args(scriptPath)...)
			if err := cmd.Start(); err != nil {
				continue
			}
			go cmd.Wait()
			started = true
			break
		}
	}

	if !started {
		os.Remove(scriptPath)
		return 0, fmt.Errorf("no terminal emulator found")
	}

	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		data, err := os.ReadFile(pidFile)
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil && pid > 0 {
				return pid, nil
			}
		}
	}

	return 0, fmt.Errorf("timed out waiting for session PID")
}
