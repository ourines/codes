package commands

import (
	"os/exec"

	"github.com/spf13/cobra"

	"codes/internal/ui"
)

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
