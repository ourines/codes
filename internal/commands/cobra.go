package commands

import (
	"os/exec"

	"github.com/spf13/cobra"

	"codes/internal/ui"
)

// InitCmd represents the init command
var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Check environment and configuration",
	Long:  "Verify Claude CLI installation and validate configuration files",
	Run: func(cmd *cobra.Command, args []string) {
		RunInit()
	},
}

// InstallCmd represents the install command
var InstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install codes CLI to system",
	Long:  "Install codes CLI to system PATH for global access",
	Run: func(cmd *cobra.Command, args []string) {
		RunInstall()
	},
}

// AddCmd represents the add command
var AddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new Claude configuration",
	Long:  "Interactively add a new Claude API configuration",
	Run: func(cmd *cobra.Command, args []string) {
		RunAdd()
	},
}

// SelectCmd represents the select command
var SelectCmd = &cobra.Command{
	Use:   "select",
	Short: "Select Claude configuration",
	Long:  "Interactively select which Claude configuration to use",
	Run: func(cmd *cobra.Command, args []string) {
		RunSelect()
	},
}

// UpdateCmd represents the update command
var UpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Claude to specific version",
	Long:  "Update Claude CLI to a specific version",
	Run: func(cmd *cobra.Command, args []string) {
		RunUpdate()
	},
}

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show codes version",
	Long:  "Show the version of codes CLI",
	Run: func(cmd *cobra.Command, args []string) {
		RunVersion()
	},
}

// RunCmd represents the default run command
var RunCmd = &cobra.Command{
	Use:  "codes",
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if claude is installed
		if _, err := exec.LookPath("claude"); err != nil {
			ui.ShowLoading("Claude CLI not found. Installing...")
			InstallClaude("latest")
			return
		}
		RunClaudeWithConfig(args)
	},
}

// StartCmd represents the start command
var StartCmd = &cobra.Command{
	Use:   "start [path-or-project-name]",
	Short: "Start Claude in a specific directory",
	Long:  "Start Claude Code in a specific directory, project alias, or last used directory",
	Run: func(cmd *cobra.Command, args []string) {
		RunStart(args)
	},
}

// ProjectCmd represents the project command
var ProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage project aliases",
	Long:  "Add, remove, or list project aliases for quick access",
}

// ProjectAddCmd represents the project add command
var ProjectAddCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Add a project alias",
	Long:  "Add a project alias for quick access to a directory",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		RunProjectAdd(args[0], args[1])
	},
}

// ProjectRemoveCmd represents the project remove command
var ProjectRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a project alias",
	Long:  "Remove a project alias",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunProjectRemove(args[0])
	},
}

// ProjectListCmd represents the project list command
var ProjectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all project aliases",
	Long:  "List all configured project aliases",
	Run: func(cmd *cobra.Command, args []string) {
		RunProjectList()
	},
}

func init() {
	ProjectCmd.AddCommand(ProjectAddCmd)
	ProjectCmd.AddCommand(ProjectRemoveCmd)
	ProjectCmd.AddCommand(ProjectListCmd)
}
