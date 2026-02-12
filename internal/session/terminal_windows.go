//go:build windows

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

// buildWindowsScript generates a PowerShell script for running a session.
func buildWindowsScript(name, dir string, args []string, env map[string]string) (script string, scriptPath string) {
	pidFile := pidFilePath(name)
	scriptPath = fmt.Sprintf("%s\\codes-%s.ps1", os.TempDir(), name)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# codes session: %s\n\n", name))

	// Write PID file
	b.WriteString(fmt.Sprintf("$PID | Out-File -FilePath '%s' -Encoding ascii\n\n",
		strings.ReplaceAll(pidFile, "'", "''")))

	// Cleanup on exit (removes PID file and this script)
	b.WriteString("try {\n")

	// Set environment variables
	for k, v := range env {
		if !validEnvVarName.MatchString(k) {
			continue
		}
		escaped := strings.ReplaceAll(v, "'", "''")
		b.WriteString(fmt.Sprintf("  $env:%s = '%s'\n", k, escaped))
	}
	if len(env) > 0 {
		b.WriteString("\n")
	}

	// Change to project directory
	escaped := strings.ReplaceAll(dir, "'", "''")
	b.WriteString(fmt.Sprintf("  Set-Location '%s'\n\n", escaped))

	// Set window title
	b.WriteString(fmt.Sprintf("  $Host.UI.RawUI.WindowTitle = 'codes: %s'\n\n", name))

	// Run claude
	if len(args) > 0 {
		quotedArgs := make([]string, len(args))
		for i, a := range args {
			quotedArgs[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(a, "'", "''"))
		}
		b.WriteString(fmt.Sprintf("  & claude %s\n", strings.Join(quotedArgs, " ")))
	} else {
		b.WriteString("  & claude\n")
	}

	// Cleanup in finally block
	b.WriteString("} finally {\n")
	b.WriteString(fmt.Sprintf("  Remove-Item -Path '%s' -ErrorAction SilentlyContinue\n",
		strings.ReplaceAll(pidFile, "'", "''")))
	b.WriteString(fmt.Sprintf("  Remove-Item -Path '%s' -ErrorAction SilentlyContinue\n",
		strings.ReplaceAll(scriptPath, "'", "''")))
	b.WriteString("}\n")

	return b.String(), scriptPath
}

// buildWindowsBatchScript generates a .bat script as a fallback for cmd.exe.
func buildWindowsBatchScript(name, dir string, args []string, env map[string]string) (script string, scriptPath string) {
	pidFile := pidFilePath(name)
	scriptPath = fmt.Sprintf("%s\\codes-%s.bat", os.TempDir(), name)

	var b strings.Builder
	b.WriteString("@echo off\n")
	b.WriteString(fmt.Sprintf("REM codes session: %s\n\n", name))

	// Set window title
	b.WriteString(fmt.Sprintf("title codes: %s\n\n", name))

	// Write PID — not directly possible in .bat, but we can use a workaround
	// We use wmic to get the current PID
	b.WriteString(fmt.Sprintf("for /f \"tokens=2 delims==\" %%%%i in ('wmic process where \"Name='cmd.exe' and CommandLine like '%%%%%%%%codes-%s.bat%%%%%%%%'\" get ProcessId /value 2^>nul') do echo %%%%i > \"%s\"\n\n",
		name, pidFile))

	// Set environment variables
	for k, v := range env {
		if !validEnvVarName.MatchString(k) {
			continue
		}
		b.WriteString(fmt.Sprintf("SET \"%s=%s\"\n", k, v))
	}
	if len(env) > 0 {
		b.WriteString("\n")
	}

	// Change to project directory
	b.WriteString(fmt.Sprintf("cd /d \"%s\"\n\n", dir))

	// Run claude
	if len(args) > 0 {
		b.WriteString(fmt.Sprintf("claude %s\n\n", strings.Join(args, " ")))
	} else {
		b.WriteString("claude\n\n")
	}

	// Cleanup
	b.WriteString(fmt.Sprintf("del \"%s\" 2>nul\n", pidFile))
	b.WriteString(fmt.Sprintf("del \"%s\" 2>nul\n", scriptPath))

	return b.String(), scriptPath
}

// buildRemoteWindowsScript generates a PowerShell script for SSH remote sessions.
func buildRemoteWindowsScript(name string, host *config.RemoteHost, project string) (script string, scriptPath string) {
	pidFile := pidFilePath(name)
	scriptPath = fmt.Sprintf("%s\\codes-%s.ps1", os.TempDir(), name)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# codes remote session: %s\n\n", name))

	// Write PID file
	b.WriteString(fmt.Sprintf("$PID | Out-File -FilePath '%s' -Encoding ascii\n\n",
		strings.ReplaceAll(pidFile, "'", "''")))

	b.WriteString("try {\n")

	// Set window title
	b.WriteString(fmt.Sprintf("  $Host.UI.RawUI.WindowTitle = 'codes: %s (remote)'\n\n", name))

	// Build SSH command arguments
	var sshArgs []string
	sshArgs = append(sshArgs, "-t") // force TTY
	sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=accept-new")
	if host.Port != 0 {
		sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", host.Port))
	}
	if host.Identity != "" {
		identity := host.Identity
		if strings.HasPrefix(identity, "~/") {
			identity = "$env:USERPROFILE" + identity[1:]
		}
		sshArgs = append(sshArgs, "-i", identity)
	}
	sshArgs = append(sshArgs, host.UserAtHost())

	// Remote command
	remoteCmd := "codes"
	if project != "" {
		escaped := strings.ReplaceAll(project, "'", "''")
		remoteCmd = fmt.Sprintf("cd '%s' && codes", escaped)
	}

	quotedArgs := make([]string, len(sshArgs))
	for i, a := range sshArgs {
		quotedArgs[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(a, "'", "''"))
	}
	b.WriteString(fmt.Sprintf("  & ssh %s '%s'\n",
		strings.Join(quotedArgs, " "),
		strings.ReplaceAll(remoteCmd, "'", "''")))

	// Cleanup
	b.WriteString("} finally {\n")
	b.WriteString(fmt.Sprintf("  Remove-Item -Path '%s' -ErrorAction SilentlyContinue\n",
		strings.ReplaceAll(pidFile, "'", "''")))
	b.WriteString(fmt.Sprintf("  Remove-Item -Path '%s' -ErrorAction SilentlyContinue\n",
		strings.ReplaceAll(scriptPath, "'", "''")))
	b.WriteString("}\n")

	return b.String(), scriptPath
}

func openInTerminal(sessionID, dir string, args []string, env map[string]string, terminal string) (int, error) {
	// Remove stale PID file
	pidFile := pidFilePath(sessionID)
	os.Remove(pidFile)

	var cmd *exec.Cmd

	switch strings.ToLower(terminal) {
	case "", "auto":
		// Auto-detect: Windows Terminal > PowerShell
		if wtPath, err := exec.LookPath("wt.exe"); err == nil {
			script, scriptPath := buildWindowsScript(sessionID, dir, args, env)
			if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
				return 0, err
			}
			cmd = exec.Command(wtPath, "new-tab", "powershell", "-NoExit", "-File", scriptPath)
		} else {
			script, scriptPath := buildWindowsScript(sessionID, dir, args, env)
			if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
				return 0, err
			}
			cmd = exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-File", scriptPath)
		}
	case "wt":
		script, scriptPath := buildWindowsScript(sessionID, dir, args, env)
		if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
			return 0, err
		}
		cmd = exec.Command("wt.exe", "new-tab", "powershell", "-NoExit", "-File", scriptPath)
	case "powershell":
		script, scriptPath := buildWindowsScript(sessionID, dir, args, env)
		if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
			return 0, err
		}
		cmd = exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-File", scriptPath)
	case "pwsh":
		script, scriptPath := buildWindowsScript(sessionID, dir, args, env)
		if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
			return 0, err
		}
		cmd = exec.Command("cmd", "/c", "start", "pwsh", "-NoExit", "-File", scriptPath)
	case "cmd":
		script, scriptPath := buildWindowsBatchScript(sessionID, dir, args, env)
		if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
			return 0, err
		}
		cmd = exec.Command("cmd", "/c", "start", "cmd", "/k", scriptPath)
	default:
		// Custom terminal command
		script, scriptPath := buildWindowsScript(sessionID, dir, args, env)
		if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
			return 0, err
		}
		cmd = exec.Command("cmd", "/c", "start", terminal, scriptPath)
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start terminal: %w", err)
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

func openRemoteInTerminal(sessionID string, host *config.RemoteHost, project string, terminal string) (int, error) {
	// Remove stale PID file
	pidFile := pidFilePath(sessionID)
	os.Remove(pidFile)

	script, scriptPath := buildRemoteWindowsScript(sessionID, host, project)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return 0, err
	}

	var cmd *exec.Cmd

	switch strings.ToLower(terminal) {
	case "", "auto":
		if _, err := exec.LookPath("wt.exe"); err == nil {
			cmd = exec.Command("wt.exe", "new-tab", "powershell", "-NoExit", "-File", scriptPath)
		} else {
			cmd = exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-File", scriptPath)
		}
	case "wt":
		cmd = exec.Command("wt.exe", "new-tab", "powershell", "-NoExit", "-File", scriptPath)
	case "powershell":
		cmd = exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-File", scriptPath)
	case "pwsh":
		cmd = exec.Command("cmd", "/c", "start", "pwsh", "-NoExit", "-File", scriptPath)
	case "cmd":
		// For cmd, we need a .bat script instead
		batScript, batPath := buildWindowsBatchScript(sessionID, "", nil, nil)
		// Rewrite with SSH command
		var b strings.Builder
		b.WriteString("@echo off\n")
		b.WriteString(fmt.Sprintf("title codes: %s (remote)\n", sessionID))

		var sshArgs []string
		sshArgs = append(sshArgs, "-t", "-o", "StrictHostKeyChecking=accept-new")
		if host.Port != 0 {
			sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", host.Port))
		}
		if host.Identity != "" {
			sshArgs = append(sshArgs, "-i", host.Identity)
		}
		sshArgs = append(sshArgs, host.UserAtHost())
		remoteCmd := "codes"
		if project != "" {
			remoteCmd = fmt.Sprintf("cd \"%s\" && codes", project)
		}
		b.WriteString(fmt.Sprintf("ssh %s \"%s\"\n", strings.Join(sshArgs, " "), remoteCmd))
		b.WriteString(fmt.Sprintf("del \"%s\" 2>nul\n", pidFilePath(sessionID)))
		b.WriteString(fmt.Sprintf("del \"%s\" 2>nul\n", batPath))
		batScript = b.String()
		_ = batScript
		os.Remove(scriptPath) // remove the .ps1 we don't need
		if err := os.WriteFile(batPath, []byte(batScript), 0644); err != nil {
			return 0, err
		}
		cmd = exec.Command("cmd", "/c", "start", "cmd", "/k", batPath)
	default:
		cmd = exec.Command("cmd", "/c", "start", terminal, scriptPath)
	}

	if err := cmd.Start(); err != nil {
		os.Remove(scriptPath)
		return 0, fmt.Errorf("failed to start terminal: %w", err)
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

func focusTerminalWindow(terminal string) {
	// Best effort: try to use PowerShell AppActivate
	var windowTitle string
	switch strings.ToLower(terminal) {
	case "", "auto", "wt":
		windowTitle = "Windows Terminal"
	case "powershell", "pwsh":
		windowTitle = "codes:"
	case "cmd":
		windowTitle = "codes:"
	default:
		return
	}

	// Use VBScript-style AppActivate via PowerShell
	script := fmt.Sprintf(
		`(New-Object -ComObject WScript.Shell).AppActivate('%s')`,
		windowTitle)
	exec.Command("powershell", "-NoProfile", "-Command", script).Run()
}

func isProcessAlive(pid int) bool {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), strconv.Itoa(pid))
}

func killProcess(pid int) error {
	return exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}
