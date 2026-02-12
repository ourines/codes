//go:build windows

package session

import (
	"os"
	"strings"
	"testing"

	"codes/internal/config"
)

func TestBuildWindowsScript_Basic(t *testing.T) {
	script, scriptPath := buildWindowsScript("test-session", "C:\\Users\\test\\project", nil, nil)

	if !strings.HasSuffix(scriptPath, "codes-test-session.ps1") {
		t.Errorf("unexpected script path: %s", scriptPath)
	}
	if !strings.Contains(script, "# codes session: test-session") {
		t.Error("script should contain session name comment")
	}
	if !strings.Contains(script, "Set-Location") {
		t.Error("script should contain Set-Location for directory change")
	}
	if !strings.Contains(script, "C:\\Users\\test\\project") {
		t.Error("script should contain the project directory")
	}
	if !strings.Contains(script, "& claude") {
		t.Error("script should run claude")
	}
	if !strings.Contains(script, "try {") {
		t.Error("script should have try block")
	}
	if !strings.Contains(script, "} finally {") {
		t.Error("script should have finally block for cleanup")
	}
	if !strings.Contains(script, "$PID | Out-File") {
		t.Error("script should write PID file")
	}
	if !strings.Contains(script, "Remove-Item") {
		t.Error("script should clean up PID file in finally block")
	}
}

func TestBuildWindowsScript_WithArgs(t *testing.T) {
	args := []string{"--model", "opus", "--verbose"}
	script, _ := buildWindowsScript("s1", "C:\\tmp", args, nil)

	for _, arg := range args {
		if !strings.Contains(script, "'"+arg+"'") {
			t.Errorf("script should contain quoted arg %q", arg)
		}
	}
	if !strings.Contains(script, "& claude") {
		t.Error("script should run claude with & operator")
	}
}

func TestBuildWindowsScript_WithEnv(t *testing.T) {
	env := map[string]string{
		"ANTHROPIC_BASE_URL":  "https://api.example.com",
		"ANTHROPIC_AUTH_TOKEN": "sk-test-123",
	}
	script, _ := buildWindowsScript("s1", "C:\\tmp", nil, env)

	if !strings.Contains(script, "$env:ANTHROPIC_BASE_URL = 'https://api.example.com'") {
		t.Error("script should set ANTHROPIC_BASE_URL env var")
	}
	if !strings.Contains(script, "$env:ANTHROPIC_AUTH_TOKEN = 'sk-test-123'") {
		t.Error("script should set ANTHROPIC_AUTH_TOKEN env var")
	}
}

func TestBuildWindowsScript_EnvInjectionPrevention(t *testing.T) {
	env := map[string]string{
		"VALID_VAR":    "safe-value",
		"INVALID-VAR":  "should-be-skipped",
		"123BAD":       "should-be-skipped",
		"$(malicious)": "should-be-skipped",
	}
	script, _ := buildWindowsScript("s1", "C:\\tmp", nil, env)

	if !strings.Contains(script, "$env:VALID_VAR = 'safe-value'") {
		t.Error("script should set VALID_VAR")
	}
	if strings.Contains(script, "INVALID-VAR") {
		t.Error("script should NOT contain INVALID-VAR")
	}
	if strings.Contains(script, "123BAD") {
		t.Error("script should NOT contain 123BAD")
	}
	if strings.Contains(script, "malicious") {
		t.Error("script should NOT contain malicious injection")
	}
}

func TestBuildWindowsScript_SingleQuoteEscaping(t *testing.T) {
	env := map[string]string{
		"TOKEN": "it's a token",
	}
	script, _ := buildWindowsScript("s1", "C:\\Users\\it's a dir", nil, env)

	// In PowerShell, single quotes are escaped by doubling them
	if !strings.Contains(script, "it''s a dir") {
		t.Error("directory path should have doubled single quotes for PS escaping")
	}
	if !strings.Contains(script, "it''s a token") {
		t.Error("env value should have doubled single quotes for PS escaping")
	}
}

func TestBuildWindowsScript_WindowTitle(t *testing.T) {
	script, _ := buildWindowsScript("my-session", "C:\\tmp", nil, nil)

	if !strings.Contains(script, "$Host.UI.RawUI.WindowTitle = 'codes: my-session'") {
		t.Error("script should set window title via $Host.UI.RawUI.WindowTitle")
	}
}

func TestBuildWindowsBatchScript_Basic(t *testing.T) {
	script, scriptPath := buildWindowsBatchScript("test-session", "C:\\Users\\test\\project", nil, nil)

	if !strings.HasSuffix(scriptPath, "codes-test-session.bat") {
		t.Errorf("unexpected script path: %s", scriptPath)
	}
	if !strings.HasPrefix(script, "@echo off") {
		t.Error("batch script should start with @echo off")
	}
	if !strings.Contains(script, "title codes: test-session") {
		t.Error("batch script should set window title")
	}
	if !strings.Contains(script, "cd /d \"C:\\Users\\test\\project\"") {
		t.Error("batch script should cd /d to project directory")
	}
	if !strings.Contains(script, "claude") {
		t.Error("batch script should run claude")
	}
	if !strings.Contains(script, "del ") {
		t.Error("batch script should clean up files")
	}
}

func TestBuildWindowsBatchScript_WithEnv(t *testing.T) {
	env := map[string]string{
		"MY_VAR": "test-value",
	}
	script, _ := buildWindowsBatchScript("s1", "C:\\tmp", nil, env)

	if !strings.Contains(script, "SET \"MY_VAR=test-value\"") {
		t.Error("batch script should SET environment variables")
	}
}

func TestBuildWindowsBatchScript_WithArgs(t *testing.T) {
	args := []string{"--model", "opus"}
	script, _ := buildWindowsBatchScript("s1", "C:\\tmp", args, nil)

	if !strings.Contains(script, "claude --model opus") {
		t.Error("batch script should pass args to claude")
	}
}

func TestBuildRemoteWindowsScript_Basic(t *testing.T) {
	host := &config.RemoteHost{
		Name: "dev",
		Host: "example.com",
		User: "deploy",
	}
	script, scriptPath := buildRemoteWindowsScript("remote-dev", host, "")

	if !strings.HasSuffix(scriptPath, "codes-remote-dev.ps1") {
		t.Errorf("unexpected script path: %s", scriptPath)
	}
	if !strings.Contains(script, "# codes remote session: remote-dev") {
		t.Error("script should contain remote session comment")
	}
	if !strings.Contains(script, "deploy@example.com") {
		t.Error("script should contain user@host")
	}
	if !strings.Contains(script, "& ssh") {
		t.Error("script should contain ssh command with & operator")
	}
	if !strings.Contains(script, "-t") {
		t.Error("script should force TTY")
	}
	if !strings.Contains(script, "StrictHostKeyChecking=accept-new") {
		t.Error("script should set StrictHostKeyChecking")
	}
	if !strings.Contains(script, "try {") {
		t.Error("script should have try block")
	}
	if !strings.Contains(script, "} finally {") {
		t.Error("script should have finally cleanup block")
	}
}

func TestBuildRemoteWindowsScript_WithPort(t *testing.T) {
	host := &config.RemoteHost{
		Name: "dev",
		Host: "example.com",
		User: "deploy",
		Port: 2222,
	}
	script, _ := buildRemoteWindowsScript("remote-dev", host, "")

	if !strings.Contains(script, "-p") || !strings.Contains(script, "2222") {
		t.Error("script should contain -p 2222 for custom port")
	}
}

func TestBuildRemoteWindowsScript_WithIdentity(t *testing.T) {
	host := &config.RemoteHost{
		Name:     "dev",
		Host:     "example.com",
		Identity: "~/.ssh/deploy_key",
	}
	script, _ := buildRemoteWindowsScript("remote-dev", host, "")

	// ~/ should be expanded to $env:USERPROFILE on Windows
	if !strings.Contains(script, "$env:USERPROFILE/.ssh/deploy_key") {
		t.Error("script should expand ~ to $env:USERPROFILE in identity path")
	}
}

func TestBuildRemoteWindowsScript_WithProject(t *testing.T) {
	host := &config.RemoteHost{
		Name: "dev",
		Host: "example.com",
		User: "deploy",
	}
	script, _ := buildRemoteWindowsScript("remote-dev", host, "/home/deploy/project")

	if !strings.Contains(script, "/home/deploy/project") {
		t.Error("script should contain project directory path")
	}
}

func TestBuildRemoteWindowsScript_WindowTitle(t *testing.T) {
	host := &config.RemoteHost{
		Name: "dev",
		Host: "example.com",
	}
	script, _ := buildRemoteWindowsScript("remote-dev", host, "")

	if !strings.Contains(script, "codes: remote-dev (remote)") {
		t.Error("script should set window title with (remote) suffix")
	}
}

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	if !isProcessAlive(os.Getpid()) {
		t.Error("current process should be alive")
	}
}

func TestIsProcessAlive_InvalidPID(t *testing.T) {
	if isProcessAlive(9999999) {
		t.Error("PID 9999999 should not be alive")
	}
}
