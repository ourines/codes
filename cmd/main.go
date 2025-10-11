package main

import (
	"os"

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
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}