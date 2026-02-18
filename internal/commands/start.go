package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"codes/internal/config"
	"codes/internal/remote"
	"codes/internal/ui"
)

// RunStart launches Claude in a target directory or project alias.
func RunStart(args []string) {
	var targetDir string

	if len(args) > 0 {
		input := args[0]

		if project, exists := config.GetProject(input); exists {
			// Remote project → SSH
			if project.Remote != "" {
				host, ok := config.GetRemote(project.Remote)
				if !ok {
					ui.ShowError(fmt.Sprintf("Remote '%s' not found for project '%s'", project.Remote, input), nil)
					os.Exit(1)
				}
				ui.ShowInfo("Connecting to remote project: %s @ %s", input, host.UserAtHost())
				if err := remote.RunSSHInteractive(host, fmt.Sprintf("cd %s && codes", project.Path)); err != nil {
					ui.ShowError("SSH session failed", err)
					os.Exit(1)
				}
				return
			}
			targetDir = project.Path
			ui.ShowInfo("Using project: %s -> %s", input, targetDir)
		} else {
			absPath, err := filepath.Abs(input)
			if err != nil {
				ui.ShowError("Invalid path", err)
				os.Exit(1)
			}
			targetDir = absPath
		}

		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			ui.ShowError("Directory does not exist", err)
			os.Exit(1)
		}
	} else {
		var err error
		behavior := config.GetDefaultBehavior()

		switch behavior {
		case "current":
			targetDir, err = os.Getwd()
			if err != nil {
				ui.ShowError("Failed to get current directory", err)
				os.Exit(1)
			}
			ui.ShowInfo("Using current directory: %s", targetDir)
		case "last":
			lastDir, err := config.GetLastWorkDir()
			if err != nil {
				ui.ShowError("Failed to get last working directory", err)
				os.Exit(1)
			}
			targetDir = lastDir
			ui.ShowInfo("Using last directory: %s", targetDir)
		case "home":
			homeDir, err := os.UserHomeDir()
			if err != nil {
				ui.ShowError("Failed to get home directory", err)
				os.Exit(1)
			}
			targetDir = homeDir
			ui.ShowInfo("Using home directory: %s", targetDir)
		default:
			targetDir, err = os.Getwd()
			if err != nil {
				ui.ShowError("Failed to get current directory", err)
				os.Exit(1)
			}
			ui.ShowInfo("Using current directory: %s", targetDir)
		}
	}

	if err := config.SaveLastWorkDir(targetDir); err != nil {
		ui.ShowWarning("Failed to save working directory: %v", err)
	}

	runClaudeInDirectory(targetDir)
}
