package main

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"codes/internal/commands"
	"codes/internal/output"
	"codes/internal/tui"
)

var jsonFlag bool

var rootCmd = &cobra.Command{
	Use:   "codes",
	Short: "A CLI tool to manage Claude environments and versions",
	Long:  "A Go-based CLI tool to manage Claude environments and versions with multi-configuration support",
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output in JSON format")

	rootCmd.AddCommand(commands.InitCmd)
	rootCmd.AddCommand(commands.UpdateCmd)
	rootCmd.AddCommand(commands.VersionCmd)
	rootCmd.AddCommand(commands.DoctorCmd)
	rootCmd.AddCommand(commands.StartCmd)
	rootCmd.AddCommand(commands.ProfileCmd)
	rootCmd.AddCommand(commands.ProjectCmd)
	rootCmd.AddCommand(commands.ConfigCmd)
	rootCmd.AddCommand(commands.CompletionCmd)
	rootCmd.AddCommand(commands.ServeCmd)
	rootCmd.AddCommand(commands.RemoteCmd)
	rootCmd.AddCommand(commands.ClaudeCmd)
	rootCmd.AddCommand(commands.AgentCmd)
	rootCmd.AddCommand(commands.TaskSimpleCmd)
	rootCmd.AddCommand(commands.WorkflowCmd)
	rootCmd.AddCommand(commands.NotifyCmd)
	rootCmd.AddCommand(commands.NotifyCmd)

	// 设置默认运行时行为
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		// Set JSON mode from flag
		output.JSONMode = jsonFlag

		// If --json flag, output project list in JSON
		if jsonFlag {
			commands.RunProjectList()
			return
		}

		// If stdin is a TTY, launch TUI (sessions managed inside TUI)
		if term.IsTerminal(int(os.Stdin.Fd())) {
			if err := tui.Run(commands.Version); err != nil {
				os.Exit(1)
			}
			return
		}

		// Non-TTY fallback: original behavior
		if _, err := exec.LookPath("claude"); err != nil {
			commands.RunClaudeWithConfig([]string{})
			return
		}
		commands.RunStart(args)
	}
}

func main() {
	// Propagate --json flag before execution
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		output.JSONMode = jsonFlag
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
