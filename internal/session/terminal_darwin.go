//go:build darwin

package session

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// openInTerminal opens a new terminal window running Claude Code.
// The terminal parameter selects which app to use: "iterm", "warp", or "" / "terminal" for Terminal.app.
func openInTerminal(sessionID, dir string, args []string, env map[string]string, terminal string) (int, error) {
	script, scriptPath := buildScript(sessionID, dir, args, env)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return 0, err
	}

	// Remove stale PID file
	pidFile := pidFilePath(sessionID)
	os.Remove(pidFile)

	// Launch in the selected terminal
	var err error
	switch strings.ToLower(terminal) {
	case "iterm", "iterm2":
		err = openITerm(scriptPath, sessionID)
	case "warp":
		err = openWarp(scriptPath)
	case "", "terminal":
		err = openTerminalApp(scriptPath)
	default:
		// Custom terminal command (e.g., "/Applications/Alacritty.app/...")
		err = openCustom(terminal, scriptPath)
	}

	if err != nil {
		os.Remove(scriptPath)
		return 0, err
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

func openTerminalApp(scriptPath string) error {
	appleScript := fmt.Sprintf(`tell application "Terminal"
	activate
	do script "bash '%s'"
end tell`, scriptPath)

	return exec.Command("osascript", "-e", appleScript).Run()
}

func openITerm(scriptPath, sessionID string) error {
	appleScript := fmt.Sprintf(`tell application "iTerm"
	activate
	set newWindow to (create window with default profile)
	tell current session of newWindow
		set name to "codes: %s"
		write text "bash '%s'"
	end tell
end tell`, sessionID, scriptPath)

	return exec.Command("osascript", "-e", appleScript).Run()
}

func openWarp(scriptPath string) error {
	// Warp doesn't have robust AppleScript support, use open command
	return exec.Command("open", "-a", "Warp", scriptPath).Run()
}

func openCustom(terminal, scriptPath string) error {
	// Try as an app name first (e.g., "Alacritty")
	if !strings.Contains(terminal, "/") {
		if err := exec.Command("open", "-a", terminal, scriptPath).Run(); err == nil {
			return nil
		}
	}
	// Try as a direct command
	return exec.Command(terminal, "-e", "bash", scriptPath).Run()
}

// focusTerminalWindow brings the configured terminal to the foreground.
func focusTerminalWindow(terminal string) {
	app := "Terminal"
	switch strings.ToLower(terminal) {
	case "iterm", "iterm2":
		app = "iTerm"
	case "warp":
		app = "Warp"
	case "", "terminal":
		app = "Terminal"
	default:
		if !strings.Contains(terminal, "/") {
			app = terminal
		} else {
			return
		}
	}
	exec.Command("osascript", "-e", fmt.Sprintf(`tell application "%s" to activate`, app)).Run()
}
