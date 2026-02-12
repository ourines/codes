package commands

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"codes/internal/config"
	"codes/internal/ui"
)

// InitCmd represents the init command
var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Check environment and configuration",
	Long:  "Verify Claude CLI installation and validate configuration files",
	Run: func(cmd *cobra.Command, args []string) {
		autoYes, _ := cmd.Flags().GetBool("yes")
		RunInit(autoYes)
	},
}

func init() {
	InitCmd.Flags().BoolP("yes", "y", false, "Auto-accept all prompts (for non-interactive use)")
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
	ValidArgsFunction: completeProfileNames,
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
	ValidArgsFunction: completeProjectNames,
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
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"defaultBehavior"}, cobra.ShellCompDirectiveNoFileComp
		}
		if len(args) == 1 && args[0] == "defaultBehavior" {
			return []string{"current", "last", "home"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		RunConfigSet(args[0], args[1])
	},
}

// ConfigGetCmd represents the config get command
var ConfigGetCmd = &cobra.Command{
	Use:       "get [key]",
	Short:     "Get configuration values",
	Long:      "Get configuration values. If no key is specified, show all configuration",
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"defaultBehavior"},
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
	Use:       "set <behavior>",
	Short:     "Set the default behavior",
	Long:      "Set the default startup behavior: 'current' (current directory), 'last' (last used directory), 'home' (home directory)",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"current", "last", "home"},
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
	Use:       "set <true|false>",
	Short:     "Set the global skipPermissions",
	Long:      "Set whether to use --dangerously-skip-permissions for all configurations that don't have their own setting",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"true", "false"},
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
	Long:  "Add a project alias for quick access to a directory. Use --remote for remote projects.",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		remoteName, _ := cmd.Flags().GetString("remote")
		RunProjectAdd(args[0], args[1], remoteName)
	},
}

// ProjectRemoveCmd represents the project remove command
var ProjectRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a project alias",
	Long:  "Remove a project alias",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: completeProjectNames,
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
	ProjectAddCmd.Flags().StringP("remote", "r", "", "Remote host name (for remote projects)")
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

	TerminalCmd.AddCommand(TerminalSetCmd)
	TerminalCmd.AddCommand(TerminalGetCmd)
	TerminalCmd.AddCommand(TerminalListCmd)

	// Remote sub-commands
	RemoteAddCmd.Flags().IntP("port", "p", 0, "SSH port")
	RemoteAddCmd.Flags().StringP("identity", "i", "", "SSH identity file")
	RemoteCmd.AddCommand(RemoteAddCmd)
	RemoteCmd.AddCommand(RemoteRemoveCmd)
	RemoteCmd.AddCommand(RemoteListCmd)
	RemoteCmd.AddCommand(RemoteStatusCmd)
	RemoteCmd.AddCommand(RemoteInstallCmd)
	RemoteCmd.AddCommand(RemoteSyncCmd)
	RemoteCmd.AddCommand(RemoteSetupCmd)
	RemoteCmd.AddCommand(RemoteSSHCmd)
}

// CompletionCmd generates shell completion scripts
// TerminalCmd represents the terminal command
var TerminalCmd = &cobra.Command{
	Use:   "terminal",
	Short: "Manage terminal emulator setting",
	Long:  "Configure which terminal emulator to use for Claude Code sessions",
}

// TerminalSetCmd sets the terminal emulator
var TerminalSetCmd = &cobra.Command{
	Use:   "set <terminal>",
	Short: "Set the terminal emulator",
	Long:  "Set which terminal emulator to use for sessions. Run 'codes terminal list' to see available options.",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if runtime.GOOS == "windows" {
			return []string{
				"auto\tAuto-detect (Windows Terminal > PowerShell)",
				"wt\tWindows Terminal",
				"powershell\tWindows PowerShell",
				"pwsh\tPowerShell Core",
				"cmd\tCommand Prompt",
			}, cobra.ShellCompDirectiveNoFileComp
		}
		return []string{
			"terminal\tTerminal.app (macOS default)",
			"iterm\tiTerm2",
			"warp\tWarp",
		}, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		RunTerminalSet(args[0])
	},
}

// TerminalGetCmd shows the current terminal emulator
var TerminalGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show current terminal emulator",
	Long:  "Show which terminal emulator is configured",
	Run: func(cmd *cobra.Command, args []string) {
		RunTerminalGet()
	},
}

// TerminalListCmd lists available terminal options
var TerminalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available terminal options",
	Long:  "List known terminal emulator options",
	Run: func(cmd *cobra.Command, args []string) {
		RunTerminalList()
	},
}

var CompletionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for the specified shell.

Usage examples:
  # Bash
  source <(codes completion bash)

  # Zsh
  source <(codes completion zsh)

  # Fish
  codes completion fish | source

  # PowerShell
  codes completion powershell | Out-String | Invoke-Expression`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return fmt.Errorf("unsupported shell: %s", args[0])
	},
}

// ServeCmd represents the serve command for MCP server mode
var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server mode",
	Long:  "Start codes as an MCP server over stdio for integration with Claude Code",
	Run: func(cmd *cobra.Command, args []string) {
		RunServe()
	},
}

// RemoteCmd represents the remote command
var RemoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage remote SSH hosts",
	Long:  "Add, remove, list, and manage remote SSH hosts for running Claude Code remotely",
}

// RemoteAddCmd adds a remote host
var RemoteAddCmd = &cobra.Command{
	Use:   "add <name> <[user@]host>",
	Short: "Add a remote host",
	Long:  "Add a remote SSH host configuration for remote Claude Code sessions",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		identity, _ := cmd.Flags().GetString("identity")
		RunRemoteAdd(args[0], args[1], port, identity)
	},
}

// RemoteRemoveCmd removes a remote host
var RemoteRemoveCmd = &cobra.Command{
	Use:               "remove <name>",
	Short:             "Remove a remote host",
	Long:              "Remove a remote SSH host configuration",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeRemoteNames,
	Run: func(cmd *cobra.Command, args []string) {
		RunRemoteRemove(args[0])
	},
}

// RemoteListCmd lists all remote hosts
var RemoteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List remote hosts",
	Long:  "List all configured remote SSH hosts",
	Run: func(cmd *cobra.Command, args []string) {
		RunRemoteList()
	},
}

// RemoteStatusCmd shows remote host status
var RemoteStatusCmd = &cobra.Command{
	Use:               "status <name>",
	Short:             "Check remote host status",
	Long:              "Check SSH connectivity and installed tools on a remote host",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeRemoteNames,
	Run: func(cmd *cobra.Command, args []string) {
		RunRemoteStatus(args[0])
	},
}

// RemoteInstallCmd installs codes on a remote host
var RemoteInstallCmd = &cobra.Command{
	Use:               "install <name>",
	Short:             "Install codes on remote host",
	Long:              "Download and install the codes binary on a remote SSH host",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeRemoteNames,
	Run: func(cmd *cobra.Command, args []string) {
		RunRemoteInstall(args[0])
	},
}

// RemoteSyncCmd syncs profiles to a remote host
var RemoteSyncCmd = &cobra.Command{
	Use:               "sync <name>",
	Short:             "Sync profiles to remote host",
	Long:              "Upload local API profiles and settings to a remote host",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeRemoteNames,
	Run: func(cmd *cobra.Command, args []string) {
		RunRemoteSync(args[0])
	},
}

// RemoteSetupCmd runs install + sync on a remote host
var RemoteSetupCmd = &cobra.Command{
	Use:               "setup <name>",
	Short:             "Full setup on remote host",
	Long:              "Install codes and sync profiles to a remote host (install + sync)",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeRemoteNames,
	Run: func(cmd *cobra.Command, args []string) {
		RunRemoteSetup(args[0])
	},
}

// RemoteSSHCmd opens an SSH session on a remote host
var RemoteSSHCmd = &cobra.Command{
	Use:               "ssh <name> [project]",
	Short:             "Open remote Claude Code session",
	Long:              "SSH into a remote host and start codes. Optionally specify a project directory.",
	Args:              cobra.RangeArgs(1, 2),
	ValidArgsFunction: completeRemoteNames,
	Run: func(cmd *cobra.Command, args []string) {
		project := ""
		if len(args) > 1 {
			project = args[1]
		}
		RunRemoteSSH(args[0], project)
	},
}

// completeProfileNames provides dynamic completion for API profile names
func completeProfileNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, c := range cfg.Profiles {
		names = append(names, c.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeProjectNames provides dynamic completion for project names
func completeProjectNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	projects, err := config.ListProjects()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for name := range projects {
		names = append(names, name)
	}
	if cmd.Name() == "remove" {
		return names, cobra.ShellCompDirectiveNoFileComp
	}
	return names, cobra.ShellCompDirectiveDefault
}

// completeRemoteNames provides dynamic completion for remote host names.
func completeRemoteNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	remotes, err := config.ListRemotes()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, r := range remotes {
		names = append(names, r.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
