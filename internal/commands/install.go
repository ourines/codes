package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"codes/internal/ui"
)

func InstallClaude(version string) {
	installClaude(version)
}

func installClaude(version string) {
	cmd := exec.Command("npm", "install", "-g", fmt.Sprintf("@anthropic-ai/claude-code@%s", version))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		ui.ShowError("Installation failed", nil)
		os.Exit(1)
	}
	ui.ShowSuccess("Claude installed successfully!")
}

// installBinary copies the codes binary to a system PATH location.
// Returns the install path and whether it was newly installed.
func installBinary() (string, bool) {
	executablePath, err := os.Executable()
	if err != nil {
		ui.ShowError("Failed to get executable path", err)
		return "", false
	}

	var targetDir string
	var installPath string

	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				ui.ShowError("Failed to get home directory", err)
				return "", false
			}
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		targetDir = filepath.Join(localAppData, "codes")
		installPath = filepath.Join(targetDir, "codes.exe")
	default:
		if ui.CanWriteTo("/usr/local/bin") {
			targetDir = "/usr/local/bin"
			installPath = filepath.Join(targetDir, "codes")
		} else {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				ui.ShowError("Failed to get home directory", err)
				return "", false
			}
			targetDir = filepath.Join(homeDir, "bin")
			installPath = filepath.Join(targetDir, "codes")
		}
	}

	executablePath, _ = filepath.EvalSymlinks(executablePath)
	targetResolved, _ := filepath.EvalSymlinks(installPath)
	if executablePath == targetResolved {
		ui.ShowSuccess("codes is already installed at %s", installPath)
		return installPath, false
	}

	ui.ShowInfo("Installing codes to: %s", installPath)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		ui.ShowError("Failed to create target directory", err)
		return "", false
	}

	src, err := os.Open(executablePath)
	if err != nil {
		ui.ShowError("Failed to read executable", err)
		return "", false
	}
	defer src.Close()

	dst, err := os.OpenFile(installPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		ui.ShowError("Failed to write to target location", err)
		return "", false
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		ui.ShowError("Failed to copy executable", err)
		return "", false
	}

	// macOS: re-sign to prevent AMFI SIGKILL on ad-hoc signed binaries
	if runtime.GOOS == "darwin" {
		exec.Command("codesign", "--force", "--sign", "-", installPath).Run()
	}

	ui.ShowSuccess("codes installed to %s", installPath)

	if runtime.GOOS == "windows" {
		ensureInPath(targetDir)
	} else if targetDir != "/usr/local/bin" {
		ui.ShowWarning("  Make sure %s is in your PATH", targetDir)
	}

	return installPath, true
}

// installShellCompletion detects the user's shell and installs completion.
func installShellCompletion() bool {
	if runtime.GOOS == "windows" {
		return installPowerShellCompletion()
	}

	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		ui.ShowWarning("Could not detect shell, skipping completion setup")
		return false
	}

	shell := filepath.Base(shellPath)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		ui.ShowError("Failed to get home directory", err)
		return false
	}

	switch shell {
	case "zsh":
		configFile := filepath.Join(homeDir, ".zshrc")
		appendCompletionLine(configFile, "source <(codes completion zsh)")
	case "bash":
		configFile := filepath.Join(homeDir, ".bashrc")
		if runtime.GOOS == "darwin" {
			configFile = filepath.Join(homeDir, ".bash_profile")
		}
		appendCompletionLine(configFile, "source <(codes completion bash)")
	case "fish":
		completionDir := filepath.Join(homeDir, ".config", "fish", "completions")
		if err := os.MkdirAll(completionDir, 0755); err != nil {
			ui.ShowError("Failed to create fish completions directory", err)
			return false
		}
		completionFile := filepath.Join(completionDir, "codes.fish")
		if _, err := os.Stat(completionFile); err == nil {
			ui.ShowSuccess("Fish completion already installed at %s", completionFile)
			return true
		}
		content := "# codes CLI completion\ncodes completion fish | source\n"
		if err := os.WriteFile(completionFile, []byte(content), 0644); err != nil {
			ui.ShowError("Failed to write fish completion", err)
			return false
		}
		ui.ShowSuccess("Fish completion installed at %s", completionFile)
	default:
		ui.ShowWarning("Unsupported shell: %s, skipping completion setup", shell)
		ui.ShowInfo("  You can manually run: codes completion --help")
		return false
	}
	return true
}

// appendCompletionLine appends a completion source line to a shell config file if not already present.
func appendCompletionLine(configFile, completionLine string) {
	if data, err := os.ReadFile(configFile); err == nil {
		if strings.Contains(string(data), "codes completion") {
			ui.ShowSuccess("Shell completion already configured in %s", configFile)
			return
		}
	}

	f, err := os.OpenFile(configFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		ui.ShowError("Failed to write to "+configFile, err)
		return
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n# codes CLI completion\n%s\n", completionLine); err != nil {
		ui.ShowError("Failed to write completion config", err)
		return
	}

	ui.ShowSuccess("Shell completion installed in %s", configFile)
}
