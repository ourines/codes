//go:build windows

package session

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

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

// escapeBatchValue escapes special characters in batch script values.
// Batch special characters: % " ^ & | ! < >
func escapeBatchValue(s string) string {
	s = strings.ReplaceAll(s, "%", "%%")   // Double percent signs
	s = strings.ReplaceAll(s, "\"", "\\\"") // Escape quotes
	s = strings.ReplaceAll(s, "^", "^^")   // Escape caret
	s = strings.ReplaceAll(s, "&", "^&")   // Escape ampersand
	s = strings.ReplaceAll(s, "|", "^|")   // Escape pipe
	s = strings.ReplaceAll(s, "!", "^!")   // Escape exclamation
	s = strings.ReplaceAll(s, "<", "^<")   // Escape less-than
	s = strings.ReplaceAll(s, ">", "^>")   // Escape greater-than
	return s
}

// buildWindowsBatchScript generates a .bat script as a fallback for cmd.exe.
func buildWindowsBatchScript(name, dir string, args []string, env map[string]string) (script string, scriptPath string) {
	pidFile := pidFilePath(name)
	scriptPath = fmt.Sprintf("%s\\codes-%s.bat", os.TempDir(), name)

	var b strings.Builder
	b.WriteString("@echo off\n")
	b.WriteString(fmt.Sprintf("REM codes session: %s\n\n", escapeBatchValue(name)))

	// Set window title
	b.WriteString(fmt.Sprintf("title codes: %s\n\n", escapeBatchValue(name)))

	// Write PID using PowerShell (WMIC is deprecated in Windows 11)
	// Use PowerShell to get the parent cmd.exe PID
	b.WriteString(fmt.Sprintf("powershell -NoProfile -Command \"$PID | Out-File -FilePath '%s' -Encoding ascii\"\n\n",
		strings.ReplaceAll(pidFile, "'", "''")))

	// Set environment variables
	for k, v := range env {
		if !validEnvVarName.MatchString(k) {
			continue
		}
		b.WriteString(fmt.Sprintf("SET \"%s=%s\"\n", k, escapeBatchValue(v)))
	}
	if len(env) > 0 {
		b.WriteString("\n")
	}

	// Change to project directory
	b.WriteString(fmt.Sprintf("cd /d \"%s\"\n\n", escapeBatchValue(dir)))

	// Run claude
	if len(args) > 0 {
		quotedArgs := make([]string, len(args))
		for i, a := range args {
			quotedArgs[i] = fmt.Sprintf("\"%s\"", escapeBatchValue(a))
		}
		b.WriteString(fmt.Sprintf("claude %s\n\n", strings.Join(quotedArgs, " ")))
	} else {
		b.WriteString("claude\n\n")
	}

	// Cleanup
	b.WriteString(fmt.Sprintf("del \"%s\" 2>nul\n", escapeBatchValue(pidFile)))
	b.WriteString(fmt.Sprintf("del \"%s\" 2>nul\n", escapeBatchValue(scriptPath)))

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
		_, batPath := buildWindowsBatchScript(sessionID, "", nil, nil)
		// Build custom SSH batch script
		var b strings.Builder
		b.WriteString("@echo off\n")
		b.WriteString(fmt.Sprintf("REM codes remote session: %s\n\n", escapeBatchValue(sessionID)))
		b.WriteString(fmt.Sprintf("title codes: %s (remote)\n\n", escapeBatchValue(sessionID)))

		// Write PID file using PowerShell
		pidFile := pidFilePath(sessionID)
		b.WriteString(fmt.Sprintf("powershell -NoProfile -Command \"$PID | Out-File -FilePath '%s' -Encoding ascii\"\n\n",
			strings.ReplaceAll(pidFile, "'", "''")))

		// Build SSH command arguments
		var sshArgs []string
		sshArgs = append(sshArgs, "-t", "-o", "StrictHostKeyChecking=accept-new")
		if host.Port != 0 {
			sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", host.Port))
		}
		if host.Identity != "" {
			identity := host.Identity
			// Expand ~ to %USERPROFILE% for batch mode
			if strings.HasPrefix(identity, "~/") {
				identity = "%USERPROFILE%" + identity[1:]
			}
			sshArgs = append(sshArgs, "-i", identity)
		}
		sshArgs = append(sshArgs, host.UserAtHost())
		remoteCmd := "codes"
		if project != "" {
			// Note: remoteCmd is used in SSH command, so it needs shell escaping, not batch escaping
			remoteCmd = fmt.Sprintf("cd \"%s\" && codes", project)
		}
		// Quote SSH args that may contain spaces (e.g. identity file paths)
		quotedSSHArgs := make([]string, len(sshArgs))
		for i, arg := range sshArgs {
			if strings.ContainsAny(arg, " %") {
				quotedSSHArgs[i] = fmt.Sprintf("\"%s\"", escapeBatchValue(arg))
			} else {
				quotedSSHArgs[i] = arg
			}
		}
		b.WriteString(fmt.Sprintf("ssh %s \"%s\"\n\n", strings.Join(quotedSSHArgs, " "), remoteCmd))

		// Cleanup
		b.WriteString(fmt.Sprintf("del \"%s\" 2>nul\n", escapeBatchValue(pidFile)))
		b.WriteString(fmt.Sprintf("del \"%s\" 2>nul\n", escapeBatchValue(batPath)))
		batScript := b.String()
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
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	const STILL_ACTIVE = 259

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProcess := kernel32.NewProc("OpenProcess")

	handle, _, _ := openProcess.Call(
		PROCESS_QUERY_LIMITED_INFORMATION,
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	var exitCode uint32
	getExitCodeProcess := kernel32.NewProc("GetExitCodeProcess")
	ret, _, _ := getExitCodeProcess.Call(uintptr(handle), uintptr(unsafe.Pointer(&exitCode)))
	if ret == 0 {
		return false
	}
	return exitCode == STILL_ACTIVE
}

func killProcess(pid int) error {
	return exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}
