//go:build windows

package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"codes/internal/ui"
)

// checkGitAvailable checks if git is installed and offers to install via winget if missing.
func checkGitAvailable() bool {
	if _, err := exec.LookPath("git"); err == nil {
		ui.ShowSuccess("Git is installed")
		return true
	}

	ui.ShowWarning("Git is not installed")

	// Check if winget is available
	if _, err := exec.LookPath("winget"); err == nil {
		ui.ShowInfo("  Git can be installed via winget.")
		fmt.Print("  Install Git now? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "y" || response == "yes" {
			ui.ShowLoading("Installing Git via winget...")
			cmd := exec.Command("winget", "install", "--id", "Git.Git", "-e",
				"--source", "winget",
				"--accept-package-agreements",
				"--accept-source-agreements")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				ui.ShowError("Failed to install Git via winget", err)
				ui.ShowInfo("  Please install Git manually: https://git-scm.com/download/win")
				return false
			}
			ui.ShowSuccess("Git installed successfully")
			ui.ShowWarning("  You may need to restart your terminal for git to be available in PATH")
			return true
		}
	}

	ui.ShowInfo("  Please install Git from: https://git-scm.com/download/win")
	return false
}

// checkExecutionPolicy checks PowerShell execution policy and offers to fix if restricted.
func checkExecutionPolicy() bool {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-ExecutionPolicy -Scope CurrentUser")
	output, err := cmd.Output()
	if err != nil {
		ui.ShowWarning("Could not check PowerShell execution policy")
		return true // non-fatal
	}

	policy := strings.TrimSpace(string(output))
	switch strings.ToLower(policy) {
	case "restricted", "allsigned":
		ui.ShowWarning("PowerShell execution policy is '%s' (scripts cannot run)", policy)
		fmt.Print("  Set execution policy to RemoteSigned? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "y" || response == "yes" {
			fixCmd := exec.Command("powershell", "-NoProfile", "-Command",
				"Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force")
			if err := fixCmd.Run(); err != nil {
				ui.ShowError("Failed to set execution policy", err)
				ui.ShowInfo("  Run manually: Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force")
				return false
			}
			ui.ShowSuccess("Execution policy set to RemoteSigned")
			return true
		}
		ui.ShowWarning("  PowerShell scripts may not work without changing execution policy")
		return false
	default:
		// RemoteSigned, Unrestricted, Bypass, Undefined — all acceptable
		ui.ShowSuccess("PowerShell execution policy: %s", policy)
		return true
	}
}

// ensureInPath adds installDir to the user's PATH if not already present.
func ensureInPath(installDir string) bool {
	// Read current user PATH
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"[Environment]::GetEnvironmentVariable('Path', 'User')")
	output, err := cmd.Output()
	if err != nil {
		ui.ShowWarning("Could not read user PATH")
		return false
	}

	currentPath := strings.TrimSpace(string(output))

	// Check if installDir is already in PATH (case-insensitive on Windows)
	for _, p := range strings.Split(currentPath, ";") {
		if strings.EqualFold(strings.TrimSpace(p), installDir) {
			ui.ShowSuccess("Installation directory already in PATH")
			return true
		}
	}

	// Add to user PATH
	newPath := installDir + ";" + currentPath
	setCmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf("[Environment]::SetEnvironmentVariable('Path', '%s', 'User')",
			strings.ReplaceAll(newPath, "'", "''")))
	if err := setCmd.Run(); err != nil {
		ui.ShowError("Failed to add to PATH", err)
		ui.ShowInfo("  Manually add %s to your PATH", installDir)
		return false
	}

	// Also update current process PATH
	currentEnv := os.Getenv("PATH")
	os.Setenv("PATH", installDir+";"+currentEnv)

	ui.ShowSuccess("Added %s to user PATH", installDir)
	ui.ShowWarning("  Restart your terminal for PATH changes to take effect")
	return true
}

// installPowerShellCompletion installs shell completion for PowerShell.
func installPowerShellCompletion() bool {
	// Get PowerShell profile path
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "$PROFILE")
	output, err := cmd.Output()
	if err != nil {
		ui.ShowWarning("Could not determine PowerShell profile path")
		return false
	}

	profilePath := strings.TrimSpace(string(output))
	if profilePath == "" {
		ui.ShowWarning("PowerShell profile path is empty")
		return false
	}

	// Create profile directory if it doesn't exist
	profileDir := filepath.Dir(profilePath)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		ui.ShowError("Failed to create PowerShell profile directory", err)
		return false
	}

	// Check if completion is already configured
	completionLine := "codes completion powershell | Out-String | Invoke-Expression"
	if data, err := os.ReadFile(profilePath); err == nil {
		if strings.Contains(string(data), "codes completion powershell") {
			ui.ShowSuccess("PowerShell completion already configured in %s", profilePath)
			return true
		}
	}

	// Append completion to profile
	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		ui.ShowError("Failed to write to PowerShell profile", err)
		return false
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n# codes CLI completion\n%s\n", completionLine); err != nil {
		ui.ShowError("Failed to write completion config", err)
		return false
	}

	ui.ShowSuccess("PowerShell completion installed in %s", profilePath)
	return true
}
