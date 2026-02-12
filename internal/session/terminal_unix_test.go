//go:build !windows

package session

import (
	"os"
	"strings"
	"testing"

	"codes/internal/config"
)

func TestBuildScript_Basic(t *testing.T) {
	script, scriptPath := buildScript("test-session", "/home/user/project", nil, nil)

	if !strings.HasSuffix(scriptPath, "codes-test-session.sh") {
		t.Errorf("unexpected script path: %s", scriptPath)
	}
	if !strings.HasPrefix(script, "#!/bin/bash") {
		t.Error("script should start with shebang")
	}
	if !strings.Contains(script, "# codes session: test-session") {
		t.Error("script should contain session name comment")
	}
	if !strings.Contains(script, "cd '/home/user/project'") {
		t.Error("script should cd to project directory")
	}
	if !strings.Contains(script, "claude\n") {
		t.Error("script should run claude")
	}
	if !strings.Contains(script, "trap cleanup EXIT") {
		t.Error("script should have cleanup trap")
	}
	if !strings.Contains(script, "echo $$ >") {
		t.Error("script should write PID file")
	}
}

func TestBuildScript_WithArgs(t *testing.T) {
	args := []string{"--model", "opus", "--verbose"}
	script, _ := buildScript("s1", "/tmp", args, nil)

	if !strings.Contains(script, "claude '--model' '--verbose'") &&
		!strings.Contains(script, "claude '--model' 'opus' '--verbose'") {
		// args order may vary but all should be present
		for _, arg := range args {
			if !strings.Contains(script, "'"+arg+"'") {
				t.Errorf("script should contain quoted arg %q", arg)
			}
		}
	}
}

func TestBuildScript_WithEnv(t *testing.T) {
	env := map[string]string{
		"ANTHROPIC_BASE_URL":  "https://api.example.com",
		"ANTHROPIC_AUTH_TOKEN": "sk-test-123",
	}
	script, _ := buildScript("s1", "/tmp", nil, env)

	if !strings.Contains(script, "export ANTHROPIC_BASE_URL='https://api.example.com'") {
		t.Error("script should export ANTHROPIC_BASE_URL")
	}
	if !strings.Contains(script, "export ANTHROPIC_AUTH_TOKEN='sk-test-123'") {
		t.Error("script should export ANTHROPIC_AUTH_TOKEN")
	}
}

func TestBuildScript_EnvInjectionPrevention(t *testing.T) {
	env := map[string]string{
		"VALID_VAR":    "safe-value",
		"INVALID-VAR":  "should-be-skipped",
		"123BAD":       "should-be-skipped",
		"$(malicious)": "should-be-skipped",
	}
	script, _ := buildScript("s1", "/tmp", nil, env)

	if !strings.Contains(script, "export VALID_VAR='safe-value'") {
		t.Error("script should export VALID_VAR")
	}
	if strings.Contains(script, "INVALID-VAR") {
		t.Error("script should NOT contain INVALID-VAR (invalid env var name)")
	}
	if strings.Contains(script, "123BAD") {
		t.Error("script should NOT contain 123BAD (starts with digit)")
	}
	if strings.Contains(script, "malicious") {
		t.Error("script should NOT contain $(malicious) injection")
	}
}

func TestBuildScript_SingleQuoteEscaping(t *testing.T) {
	env := map[string]string{
		"TOKEN": "it's a token",
	}
	script, _ := buildScript("s1", "/home/user/it's a dir", nil, env)

	// Single quotes should be escaped as '\'' in bash
	if !strings.Contains(script, "it'\\''s a dir") {
		t.Error("directory path should have escaped single quotes")
	}
	if !strings.Contains(script, "it'\\''s a token") {
		t.Error("env value should have escaped single quotes")
	}
}

func TestBuildScript_WindowTitle(t *testing.T) {
	script, _ := buildScript("my-session", "/tmp", nil, nil)

	if !strings.Contains(script, "codes: my-session") {
		t.Error("script should set window title with session name")
	}
}

func TestBuildRemoteScript_Basic(t *testing.T) {
	host := &config.RemoteHost{
		Name: "dev",
		Host: "example.com",
		User: "deploy",
	}
	script, scriptPath := buildRemoteScript("remote-dev", host, "")

	if !strings.HasSuffix(scriptPath, "codes-remote-dev.sh") {
		t.Errorf("unexpected script path: %s", scriptPath)
	}
	if !strings.Contains(script, "#!/bin/bash") {
		t.Error("script should start with shebang")
	}
	if !strings.Contains(script, "# codes remote session: remote-dev") {
		t.Error("script should contain remote session name")
	}
	if !strings.Contains(script, "deploy@example.com") {
		t.Error("script should contain user@host")
	}
	if !strings.Contains(script, "ssh") {
		t.Error("script should contain ssh command")
	}
	if !strings.Contains(script, "-t") {
		t.Error("script should force TTY with -t")
	}
	if !strings.Contains(script, "StrictHostKeyChecking=accept-new") {
		t.Error("script should set StrictHostKeyChecking")
	}
}

func TestBuildRemoteScript_WithPort(t *testing.T) {
	host := &config.RemoteHost{
		Name: "dev",
		Host: "example.com",
		User: "deploy",
		Port: 2222,
	}
	script, _ := buildRemoteScript("remote-dev", host, "")

	if !strings.Contains(script, "-p") || !strings.Contains(script, "2222") {
		t.Error("script should contain -p 2222 for custom port")
	}
}

func TestBuildRemoteScript_WithIdentity(t *testing.T) {
	host := &config.RemoteHost{
		Name:     "dev",
		Host:     "example.com",
		Identity: "~/.ssh/deploy_key",
	}
	script, _ := buildRemoteScript("remote-dev", host, "")

	// ~/  should be expanded to $HOME
	if !strings.Contains(script, "$HOME/.ssh/deploy_key") {
		t.Error("script should expand ~ to $HOME in identity path")
	}
	if !strings.Contains(script, "-i") {
		t.Error("script should contain -i flag for identity")
	}
}

func TestBuildRemoteScript_WithProject(t *testing.T) {
	host := &config.RemoteHost{
		Name: "dev",
		Host: "example.com",
		User: "deploy",
	}
	script, _ := buildRemoteScript("remote-dev", host, "/home/deploy/myproject")

	// The cd command is inside the SSH command string, so quotes are escaped
	if !strings.Contains(script, "/home/deploy/myproject") {
		t.Error("script should contain project directory path")
	}
	if !strings.Contains(script, "codes") {
		t.Error("script should run codes after cd")
	}
}

func TestBuildRemoteScript_WindowTitle(t *testing.T) {
	host := &config.RemoteHost{
		Name: "dev",
		Host: "example.com",
	}
	script, _ := buildRemoteScript("remote-dev", host, "")

	if !strings.Contains(script, "codes: remote-dev (remote)") {
		t.Error("script should set window title with (remote) suffix")
	}
}

func TestBuildRemoteScript_HostWithoutUser(t *testing.T) {
	host := &config.RemoteHost{
		Name: "dev",
		Host: "example.com",
	}
	script, _ := buildRemoteScript("remote-dev", host, "")

	if !strings.Contains(script, "'example.com'") {
		t.Error("script should contain host without user@ prefix")
	}
}

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	// Our own process should be alive
	if !isProcessAlive(os.Getpid()) {
		t.Error("current process should be alive")
	}
}

func TestIsProcessAlive_InvalidPID(t *testing.T) {
	// PID 0 or very large PID should not be alive (for regular user)
	if isProcessAlive(9999999) {
		t.Error("PID 9999999 should not be alive")
	}
}
