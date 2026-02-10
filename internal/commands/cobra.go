package commands

import (
	"os/exec"

	"github.com/spf13/cobra"

	"codes/internal/ui"
	"strings"
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

// TestCmd represents the test command
var TestCmd = &cobra.Command{
	Use:   "test [config-name]",
	Short: "Test API configuration",
	Long:  "Test API connectivity for all configurations or a specific one",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunTest(args)
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

// ConfigCmd represents the config command
var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  "Configure codes CLI settings",
}

// ConfigSetCmd represents the config set command
var ConfigSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long:  "Set a configuration value (keys: defaultBehavior)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		RunConfigSet(args[0], args[1])
	},
}

// ConfigGetCmd represents the config get command
var ConfigGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get configuration values",
	Long:  "Get configuration values. If no key is specified, show all configuration",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunConfigGet(args)
	},
}

// DefaultBehaviorCmd represents the default behavior command
var DefaultBehaviorCmd = &cobra.Command{
	Use:   "defaultbehavior",
	Short: "Manage default behavior setting",
	Long:  "Configure what directory to use when starting Claude without arguments",
}

// DefaultBehaviorSetCmd represents the defaultbehavior set command
var DefaultBehaviorSetCmd = &cobra.Command{
	Use:   "set <behavior>",
	Short: "Set the default behavior",
	Long:  "Set the default startup behavior: 'current' (current directory), 'last' (last used directory), 'home' (home directory)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunDefaultBehaviorSet(args[0])
	},
}

// DefaultBehaviorGetCmd represents the defaultbehavior get command
var DefaultBehaviorGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get the current default behavior",
	Long:  "Show the current default behavior setting",
	Run: func(cmd *cobra.Command, args []string) {
		RunDefaultBehaviorGet()
	},
}

// DefaultBehaviorResetCmd represents the defaultbehavior reset command
var DefaultBehaviorResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset to default behavior",
	Long:  "Reset the default behavior to 'current' (default)",
	Run: func(cmd *cobra.Command, args []string) {
		RunDefaultBehaviorReset()
	},
}

// SkipPermissionsCmd represents the skippermissions command
var SkipPermissionsCmd = &cobra.Command{
	Use:   "skippermissions",
	Short: "Manage global skipPermissions setting",
	Long:  "Configure the global skipPermissions setting for all Claude configurations",
}

// SkipPermissionsSetCmd represents the skippermissions set command
var SkipPermissionsSetCmd = &cobra.Command{
	Use:   "set <true|false>",
	Short: "Set the global skipPermissions",
	Long:  "Set whether to use --dangerously-skip-permissions for all configurations that don't have their own setting",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		value := strings.ToLower(args[0])
		var skip bool
		switch value {
		case "true", "t", "yes", "y", "1":
			skip = true
		case "false", "f", "no", "n", "0":
			skip = false
		default:
			ui.ShowError("Invalid value. Must be 'true' or 'false' (case-insensitive)", nil)
			return
		}
		RunSkipPermissionsSet(skip)
	},
}

// SkipPermissionsGetCmd represents the skippermissions get command
var SkipPermissionsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get the global skipPermissions setting",
	Long:  "Show the current global skipPermissions setting",
	Run: func(cmd *cobra.Command, args []string) {
		RunSkipPermissionsGet()
	},
}

// SkipPermissionsResetCmd represents the skippermissions reset command
var SkipPermissionsResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset global skipPermissions",
	Long:  "Reset the global skipPermissions to false (default)",
	Run: func(cmd *cobra.Command, args []string) {
		RunSkipPermissionsReset()
	},
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

	ConfigCmd.AddCommand(ConfigSetCmd)
	ConfigCmd.AddCommand(ConfigGetCmd)

	DefaultBehaviorCmd.AddCommand(DefaultBehaviorSetCmd)
	DefaultBehaviorCmd.AddCommand(DefaultBehaviorGetCmd)
	DefaultBehaviorCmd.AddCommand(DefaultBehaviorResetCmd)

	SkipPermissionsCmd.AddCommand(SkipPermissionsSetCmd)
	SkipPermissionsCmd.AddCommand(SkipPermissionsGetCmd)
	SkipPermissionsCmd.AddCommand(SkipPermissionsResetCmd)
}
