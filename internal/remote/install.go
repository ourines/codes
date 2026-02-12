package remote

import (
	"fmt"
	"strings"

	"codes/internal/config"
)

// RemoteStatus holds the status of a remote host.
type RemoteStatus struct {
	CodesInstalled  bool   `json:"codesInstalled"`
	CodesVersion    string `json:"codesVersion,omitempty"`
	ClaudeInstalled bool   `json:"claudeInstalled"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
}

// CheckRemoteStatus gathers installation and platform info from the remote host.
func CheckRemoteStatus(host *config.RemoteHost) (*RemoteStatus, error) {
	status := &RemoteStatus{}

	// Collect all info in one SSH call.
	// Source shell profiles first to get full PATH (npm global bin, ~/bin, etc.)
	// Use "command -v" instead of "which" (POSIX builtin, always available).
	// Use ";" instead of "&&" so each line runs independently.
	// End with "true" to guarantee exit code 0.
	script := `
for rc in ~/.bashrc ~/.profile ~/.zshrc ~/.bash_profile; do
    [ -f "$rc" ] && . "$rc" 2>/dev/null
done
export PATH="$HOME/bin:$HOME/.local/bin:$HOME/.npm-global/bin:$PATH"
echo "OS=$(uname -s)"; echo "ARCH=$(uname -m)"; echo "CODES=$(command -v codes >/dev/null 2>&1 && codes version 2>/dev/null || echo 'not found')"; echo "CLAUDE=$(command -v claude >/dev/null 2>&1 && echo 'installed' || echo 'not found')"; true`

	out, err := RunSSH(host, script)
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "OS="):
			status.OS = strings.TrimPrefix(line, "OS=")
		case strings.HasPrefix(line, "ARCH="):
			status.Arch = strings.TrimPrefix(line, "ARCH=")
		case strings.HasPrefix(line, "CODES="):
			val := strings.TrimPrefix(line, "CODES=")
			if val != "not found" && val != "" {
				status.CodesInstalled = true
				status.CodesVersion = val
			}
		case strings.HasPrefix(line, "CLAUDE="):
			val := strings.TrimPrefix(line, "CLAUDE=")
			if val != "not found" {
				status.ClaudeInstalled = true
			}
		}
	}

	return status, nil
}

// InstallOnRemote installs the codes binary on the remote host.
// Uses a non-interactive script (no sudo, no init) to avoid hanging over SSH.
// Returns the install script output along with any error.
func InstallOnRemote(host *config.RemoteHost) (string, error) {
	// First detect platform via CheckRemoteStatus
	status, err := CheckRemoteStatus(host)
	if err != nil {
		return "", fmt.Errorf("detect platform: %w", err)
	}

	goOS := normalizeOS(status.OS)
	goArch := normalizeArch(status.Arch)
	if goOS == "" || goArch == "" {
		return "", fmt.Errorf("unsupported platform: %s/%s", status.OS, status.Arch)
	}

	downloadURL := fmt.Sprintf(
		"https://github.com/ourines/codes/releases/latest/download/codes-%s-%s",
		goOS, goArch,
	)

	// Non-interactive install: download to ~/bin, no sudo, no init
	installScript := fmt.Sprintf(`
set -e
mkdir -p ~/bin
curl -fsSL '%s' -o ~/bin/codes
chmod +x ~/bin/codes

# Ensure ~/bin is in PATH for future logins
if ! echo "$PATH" | grep -q "$HOME/bin"; then
    for rc in ~/.bashrc ~/.profile ~/.zshrc; do
        if [ -f "$rc" ]; then
            echo 'export PATH="$HOME/bin:$PATH"' >> "$rc"
            break
        fi
    done
fi

~/bin/codes version
`, downloadURL)

	out, err := RunSSH(host, installScript)
	if err != nil {
		return out, fmt.Errorf("remote install failed: %w", err)
	}

	return out, nil
}

// InstallClaudeOnRemote installs Claude CLI (@anthropic-ai/claude-code) on the remote host via npm.
// Returns the install output along with any error.
func InstallClaudeOnRemote(host *config.RemoteHost) (string, error) {
	// Source shell profiles to get full PATH
	profileSetup := `
for rc in ~/.bashrc ~/.profile ~/.zshrc ~/.bash_profile; do
    [ -f "$rc" ] && . "$rc" 2>/dev/null
done
export PATH="$HOME/bin:$HOME/.local/bin:$HOME/.npm-global/bin:$PATH"
# Load nvm if available
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
`

	// Check current state
	checkScript := profileSetup + `
command -v npm >/dev/null 2>&1 && echo "npm_ok" || echo "npm_missing"
command -v claude >/dev/null 2>&1 && echo "claude_ok" || echo "claude_missing"
true`

	checkOut, err := RunSSH(host, checkScript)
	if err != nil {
		return "", fmt.Errorf("check remote environment: %w", err)
	}

	if strings.Contains(checkOut, "claude_ok") {
		return "claude already installed", nil
	}

	// If npm is missing, install Node.js via nvm (no sudo required)
	if strings.Contains(checkOut, "npm_missing") {
		nvmScript := `
set -e
export NVM_DIR="$HOME/.nvm"
if [ ! -s "$NVM_DIR/nvm.sh" ]; then
    curl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
fi
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
nvm install --lts 2>&1
echo "NODE_INSTALLED"
`
		nvmOut, err := RunSSH(host, nvmScript)
		if err != nil {
			return nvmOut, fmt.Errorf("install Node.js: %w", err)
		}
		if !strings.Contains(nvmOut, "NODE_INSTALLED") {
			return nvmOut, fmt.Errorf("Node.js installation did not complete")
		}
	}

	// Install Claude CLI globally
	installScript := profileSetup + `
npm install -g @anthropic-ai/claude-code 2>&1
`
	out, err := RunSSH(host, installScript)
	if err != nil {
		return out, fmt.Errorf("claude install failed: %w", err)
	}

	return out, nil
}

// normalizeOS converts uname -s output to Go's GOOS naming.
func normalizeOS(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "linux":
		return "linux"
	case "darwin":
		return "darwin"
	default:
		return ""
	}
}

// normalizeArch converts uname -m output to Go's GOARCH naming.
func normalizeArch(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	default:
		return ""
	}
}
