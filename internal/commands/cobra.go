package commands

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

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

// ProfileCmd represents the profile parent command
var ProfileCmd = &cobra.Command{
	Use:     "profile",
	Aliases: []string{"pf"},
	Short:   "Manage API profiles",
	Long:    "Add, select, test, list, or remove API profiles",
}

// AddCmd represents the profile add command
var AddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new Claude configuration",
	Long:  "Interactively add a new Claude API configuration",
	Run: func(cmd *cobra.Command, args []string) {
		RunAdd()
	},
}

// SelectCmd represents the profile select command
var SelectCmd = &cobra.Command{
	Use:   "select",
	Short: "Select Claude configuration",
	Long:  "Interactively select which Claude configuration to use",
	Run: func(cmd *cobra.Command, args []string) {
		RunSelect()
	},
}

// TestCmd represents the profile test command
var TestCmd = &cobra.Command{
	Use:               "test [config-name]",
	Short:             "Test API configuration",
	Long:              "Test API connectivity for all configurations or a specific one",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeProfileNames,
	Run: func(cmd *cobra.Command, args []string) {
		RunTest(args)
	},
}

// ProfileListCmd represents the profile list command
var ProfileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	Long:  "List all configured API profiles and their status",
	Run: func(cmd *cobra.Command, args []string) {
		RunProfileList()
	},
}

// ProfileRemoveCmd represents the profile remove command
var ProfileRemoveCmd = &cobra.Command{
	Use:               "remove <name>",
	Short:             "Remove a profile",
	Long:              "Remove an API profile by name",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProfileNames,
	Run: func(cmd *cobra.Command, args []string) {
		RunProfileRemove(args[0])
	},
}

// UpdateCmd represents the update command
var UpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update codes to the latest version",
	Long:  "Check for and install the latest version of codes CLI",
	Run: func(cmd *cobra.Command, args []string) {
		RunSelfUpdate()
	},
}

// ClaudeCmd is the parent command for Claude CLI management.
var ClaudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Manage Claude CLI",
	Long:  "Manage the Claude CLI (npm @anthropic-ai/claude-code) installation and versions",
}

// ClaudeUpdateCmd updates the Claude CLI npm package.
var ClaudeUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Claude CLI to specific version",
	Long:  "Update Claude CLI (npm @anthropic-ai/claude-code) to a specific version",
	Run: func(cmd *cobra.Command, args []string) {
		RunClaudeUpdate()
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

// DoctorCmd represents the doctor command
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run system diagnostics",
	Long:  "Run comprehensive system diagnostics to check Claude CLI installation, configuration, API connectivity, and agent status",
	Run: func(cmd *cobra.Command, args []string) {
		RunDoctor()
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
	Use:               "start [path-or-project-name]",
	Aliases:           []string{"s"},
	Short:             "Start Claude in a specific directory",
	Long:              "Start Claude Code in a specific directory, project alias, or last used directory",
	ValidArgsFunction: completeProjectNames,
	Run: func(cmd *cobra.Command, args []string) {
		RunStart(args)
	},
}

// ProjectCmd represents the project command
var ProjectCmd = &cobra.Command{
	Use:     "project",
	Aliases: []string{"p"},
	Short:   "Manage project aliases",
	Long:    "Add, remove, or list project aliases for quick access",
}

// ConfigCmd represents the config command
var ConfigCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"c"},
	Short:   "Manage configuration",
	Long:    "Configure codes CLI settings (default-behavior, skip-permissions, terminal)",
}

// ConfigSetCmd represents the config set command
var ConfigSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long:  "Set a configuration value (keys: default-behavior, skip-permissions, terminal, auto-update)",
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"default-behavior", "skip-permissions", "terminal", "auto-update"}, cobra.ShellCompDirectiveNoFileComp
		}
		if len(args) == 1 {
			switch args[0] {
			case "default-behavior":
				return []string{"current", "last", "home"}, cobra.ShellCompDirectiveNoFileComp
			case "skip-permissions":
				return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
			case "terminal":
				if runtime.GOOS == "windows" {
					return []string{"auto", "wt", "powershell", "pwsh", "cmd"}, cobra.ShellCompDirectiveNoFileComp
				}
				return []string{"terminal", "iterm", "warp"}, cobra.ShellCompDirectiveNoFileComp
			case "auto-update":
				return []string{"notify", "silent", "off"}, cobra.ShellCompDirectiveNoFileComp
			}
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
	ValidArgs: []string{"default-behavior", "skip-permissions", "terminal"},
	Run: func(cmd *cobra.Command, args []string) {
		RunConfigGet(args)
	},
}

// ConfigResetCmd represents the config reset command
var ConfigResetCmd = &cobra.Command{
	Use:       "reset [key]",
	Short:     "Reset configuration to defaults",
	Long:      "Reset a configuration value to its default. If no key is specified, reset all settings.",
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"default-behavior", "skip-permissions", "terminal"},
	Run: func(cmd *cobra.Command, args []string) {
		RunConfigReset(args)
	},
}

// ConfigListCmd represents the config list command
var ConfigListCmd = &cobra.Command{
	Use:       "list [key]",
	Short:     "List available values for a configuration key",
	Long:      "List available values for a configuration key (e.g., terminal options)",
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"default-behavior", "skip-permissions", "terminal"},
	Run: func(cmd *cobra.Command, args []string) {
		RunConfigList(args)
	},
}

// ConfigExportCmd represents the config export command
var ConfigExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export configuration to stdout or file",
	Long:  "Export configuration as JSON. Sensitive values (TOKEN, KEY, SECRET, PASSWORD) are redacted.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		file, _ := cmd.Flags().GetString("file")
		RunConfigExport(file)
	},
}

// ConfigImportCmd represents the config import command
var ConfigImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import configuration from a file",
	Long:  "Import configuration from a JSON file, merging with existing config. Redacted values are skipped.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunConfigImport(args[0])
	},
}

// ProjectAddCmd represents the project add command
var ProjectAddCmd = &cobra.Command{
	Use:   "add [name] [path]",
	Short: "Add a project alias",
	Long: `Add a project alias for quick access to a directory.

With no arguments, uses the current directory name and path.
With one argument, uses it as path and derives name from directory.
With two arguments, uses them as name and path.

Use --remote for remote projects.`,
	Args: cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		remoteName, _ := cmd.Flags().GetString("remote")
		RunProjectAdd2(args, remoteName)
	},
}

// ProjectRemoveCmd represents the project remove command
var ProjectRemoveCmd = &cobra.Command{
	Use:               "remove <name>",
	Short:             "Remove a project alias",
	Long:              "Remove a project alias",
	Args:              cobra.ExactArgs(1),
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

// ProjectScanCmd represents the project scan command
var ProjectScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan and import Claude projects",
	Long:  "Scan ~/.claude/projects/ for existing Claude Code projects and import them as project aliases",
	Run: func(cmd *cobra.Command, args []string) {
		RunProjectScan()
	},
}

// ProjectLinkCmd links two projects for cross-project context sharing.
var ProjectLinkCmd = &cobra.Command{
	Use:               "link <project> <linked-project>",
	Short:             "Link two projects",
	Long:              "Create a link between two projects for cross-project context sharing",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeProjectNames,
	Run: func(cmd *cobra.Command, args []string) {
		role, _ := cmd.Flags().GetString("role")
		RunProjectLink(args[0], args[1], role)
	},
}

// ProjectUnlinkCmd removes a link between two projects.
var ProjectUnlinkCmd = &cobra.Command{
	Use:               "unlink <project> <linked-project>",
	Short:             "Unlink two projects",
	Long:              "Remove the link between two projects",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeProjectNames,
	Run: func(cmd *cobra.Command, args []string) {
		RunProjectUnlink(args[0], args[1])
	},
}

func init() {
	ProjectLinkCmd.Flags().StringP("role", "r", "", "Role of the linked project (e.g. 'API provider')")
}

func init() {
	ProjectAddCmd.Flags().StringP("remote", "r", "", "Remote host name (for remote projects)")
	ProjectCmd.AddCommand(ProjectAddCmd)
	ProjectCmd.AddCommand(ProjectRemoveCmd)
	ProjectCmd.AddCommand(ProjectListCmd)
	ProjectCmd.AddCommand(ProjectScanCmd)
	ProjectCmd.AddCommand(ProjectLinkCmd)
	ProjectCmd.AddCommand(ProjectUnlinkCmd)

	ProfileCmd.AddCommand(AddCmd, SelectCmd, TestCmd, ProfileListCmd, ProfileRemoveCmd)

	ConfigCmd.AddCommand(ConfigSetCmd)
	ConfigCmd.AddCommand(ConfigGetCmd)
	ConfigCmd.AddCommand(ConfigResetCmd)
	ConfigCmd.AddCommand(ConfigListCmd)
	ConfigCmd.AddCommand(ConfigExportCmd)
	ConfigCmd.AddCommand(ConfigImportCmd)

	// Claude sub-commands
	ClaudeCmd.AddCommand(ClaudeUpdateCmd)

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
var CompletionCmd = &cobra.Command{
	Use:    "completion [bash|zsh|fish|powershell]",
	Short:  "Generate shell completion script",
	Hidden: true,
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
	Use:     "remote",
	Aliases: []string{"r"},
	Short:   "Manage remote SSH hosts",
	Long:    "Add, remove, list, and manage remote SSH hosts for running Claude Code remotely",
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
