//go:build !windows

package commands

import (
	"os/exec"
	"runtime"

	"codes/internal/ui"
)

// checkGitAvailable checks if git is installed and suggests installation via package manager.
func checkGitAvailable(autoYes bool) bool {
	if _, err := exec.LookPath("git"); err == nil {
		ui.ShowSuccess("Git is installed")
		return true
	}

	ui.ShowWarning("Git is not installed")
	switch runtime.GOOS {
	case "darwin":
		ui.ShowInfo("  Install via: xcode-select --install")
		ui.ShowInfo("  Or: brew install git")
	case "linux":
		ui.ShowInfo("  Install via your package manager:")
		ui.ShowInfo("    Ubuntu/Debian: sudo apt install git")
		ui.ShowInfo("    Fedora: sudo dnf install git")
		ui.ShowInfo("    Arch: sudo pacman -S git")
	default:
		ui.ShowInfo("  Please install Git from: https://git-scm.com/downloads")
	}
	return false
}

// checkExecutionPolicy is a no-op on non-Windows platforms.
func checkExecutionPolicy(autoYes bool) bool {
	return true
}

// ensureInPath is a no-op on non-Windows platforms (handled by shell profile).
func ensureInPath(_ string) bool {
	return true
}

// installPowerShellCompletion is not applicable on non-Windows platforms.
func installPowerShellCompletion() bool {
	return false
}
