package main

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"codes/internal/commands"
)

var rootCmd = &cobra.Command{
	Use:   "codes",
	Short: "A CLI tool to manage Claude environments and versions",
	Long:  "A Go-based CLI tool to manage Claude environments and versions with multi-configuration support",
}

func init() {
	rootCmd.AddCommand(commands.InstallCmd)
	rootCmd.AddCommand(commands.AddCmd)
	rootCmd.AddCommand(commands.SelectCmd)
	rootCmd.AddCommand(commands.UpdateCmd)
	rootCmd.AddCommand(commands.VersionCmd)

	// 设置默认运行时行为
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		// Check if claude is installed
		if _, err := exec.LookPath("claude"); err != nil {
			commands.RunClaudeWithConfig(nil)
			return
		}
		commands.RunClaudeWithConfig(args)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
