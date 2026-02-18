package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"codes/internal/config"
	"codes/internal/output"
	"codes/internal/ui"
)

// RunProjectAdd adds a project alias.
func RunProjectAdd(name, path string, remoteName string) {
	entry := config.ProjectEntry{Path: path, Remote: remoteName}

	if remoteName != "" {
		if _, ok := config.GetRemote(remoteName); !ok {
			ui.ShowError(fmt.Sprintf("Remote '%s' not found. Add it first with: codes remote add", remoteName), nil)
			return
		}

		if err := config.AddProjectEntry(name, entry); err != nil {
			ui.ShowError("Failed to add project", err)
			return
		}

		ui.ShowSuccess("Remote project '%s' added successfully!", name)
		ui.ShowInfo("Path: %s (on %s)", path, remoteName)
		ui.ShowInfo("Usage: codes start %s", name)
		return
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		ui.ShowError("Invalid path", err)
		return
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		ui.ShowError("Directory does not exist", err)
		return
	}

	entry.Path = absPath
	if err := config.AddProjectEntry(name, entry); err != nil {
		ui.ShowError("Failed to add project", err)
		return
	}

	ui.ShowSuccess("Project '%s' added successfully!", name)
	ui.ShowInfo("Path: %s", absPath)
	ui.ShowInfo("Usage: codes start %s", name)
}

// RunProjectRemove removes a project alias.
func RunProjectRemove(name string) {
	if _, exists := config.GetProjectPath(name); !exists {
		ui.ShowWarning("Project '%s' does not exist", name)
		return
	}

	if err := config.RemoveProject(name); err != nil {
		ui.ShowError("Failed to remove project", err)
		return
	}

	ui.ShowSuccess("Project '%s' removed successfully!", name)
}

// RunProjectList lists all configured projects.
func RunProjectList() {
	projects, err := config.ListProjects()
	if err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Failed to load projects", err)
		return
	}

	if output.JSONMode {
		infos := make([]config.ProjectInfo, 0, len(projects))
		for name, entry := range projects {
			infos = append(infos, config.GetProjectInfoFromEntry(name, entry))
		}
		output.Print(infos, nil)
		return
	}

	if len(projects) == 0 {
		ui.ShowInfo("No projects configured yet")
		ui.ShowInfo("Add a project with: codes project add [name] [path]")
		return
	}

	fmt.Println()
	ui.ShowHeader("Configured Projects")
	fmt.Println()

	i := 1
	for name, entry := range projects {
		if entry.Remote != "" {
			ui.ShowInfo("%d. %s -> %s @ %s", i, name, entry.Path, entry.Remote)
		} else if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
			ui.ShowWarning("%d. %s -> %s (not found)", i, name, entry.Path)
		} else {
			ui.ShowInfo("%d. %s -> %s", i, name, entry.Path)
		}
		i++
	}

	fmt.Println()
	ui.ShowInfo("Start a project with: codes start <name>")
}

// RunProjectScan scans for existing Claude Code projects and imports them.
func RunProjectScan() {
	ui.ShowLoading("Scanning ~/.claude/projects/...")

	discovered, err := config.ScanClaudeProjects()
	if err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Failed to scan Claude projects", err)
		return
	}

	if len(discovered) == 0 {
		if output.JSONMode {
			output.Print(map[string]int{"added": 0, "skipped": 0}, nil)
			return
		}
		ui.ShowInfo("No Claude projects found in ~/.claude/projects/")
		return
	}

	added, skipped, err := config.ImportDiscoveredProjects(discovered)
	if err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Failed to import projects", err)
		return
	}

	if output.JSONMode {
		output.Print(map[string]int{"added": added, "skipped": skipped, "total": len(discovered)}, nil)
		return
	}

	fmt.Println()
	ui.ShowHeader("Claude Project Scan")
	fmt.Println()

	if added > 0 {
		ui.ShowSuccess("Imported %d new project(s)", added)
	}
	if skipped > 0 {
		ui.ShowInfo("Skipped %d (already in config)", skipped)
	}
	if added == 0 {
		ui.ShowInfo("All discovered projects are already configured")
	}
	fmt.Println()
}

// RunProjectLink creates a link between two projects.
func RunProjectLink(project, linkedProject, role string) {
	if err := config.LinkProject(project, linkedProject, role); err != nil {
		ui.ShowError("Failed to link projects", err)
		return
	}
	msg := fmt.Sprintf("Linked %s → %s", project, linkedProject)
	if role != "" {
		msg += fmt.Sprintf(" (role: %s)", role)
	}
	ui.ShowSuccess("%s", msg)
}

// RunProjectUnlink removes a link between two projects.
func RunProjectUnlink(project, linkedProject string) {
	if err := config.UnlinkProject(project, linkedProject); err != nil {
		ui.ShowError("Failed to unlink projects", err)
		return
	}
	ui.ShowSuccess("Unlinked %s → %s", project, linkedProject)
}

// RunProjectAdd2 parses 0/1/2 args and calls RunProjectAdd.
func RunProjectAdd2(args []string, remoteName string) {
	var name, path string

	switch len(args) {
	case 0:
		cwd, err := os.Getwd()
		if err != nil {
			ui.ShowError("Failed to get current directory", err)
			return
		}
		path = cwd
		name = filepath.Base(cwd)
	case 1:
		absPath, err := filepath.Abs(args[0])
		if err != nil {
			ui.ShowError("Invalid path", err)
			return
		}
		path = absPath
		name = filepath.Base(absPath)
	case 2:
		name = args[0]
		path = args[1]
	}

	RunProjectAdd(name, path, remoteName)
}
