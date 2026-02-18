package commands

import (
	"fmt"
	"runtime"
	"strings"

	"codes/internal/config"
	"codes/internal/ui"
)

// RunConfigSet sets a configuration value.
func RunConfigSet(key, value string) {
	switch key {
	case "default-behavior", "defaultBehavior":
		RunDefaultBehaviorSet(value)
	case "skip-permissions", "skipPermissions":
		v := strings.ToLower(value)
		var skip bool
		switch v {
		case "true", "t", "yes", "y", "1":
			skip = true
		case "false", "f", "no", "n", "0":
			skip = false
		default:
			ui.ShowError("Invalid value for skip-permissions. Must be 'true' or 'false'", nil)
			return
		}
		RunSkipPermissionsSet(skip)
	case "terminal":
		RunTerminalSet(value)
	case "auto-update", "autoUpdate":
		v := strings.ToLower(value)
		switch v {
		case "notify", "silent", "off":
			if err := config.SetAutoUpdate(v); err != nil {
				ui.ShowError("Failed to set auto-update", err)
				return
			}
			ui.ShowSuccess("auto-update set to: %s", v)
		default:
			ui.ShowError("Invalid value for auto-update. Must be 'notify', 'silent', or 'off'", nil)
		}
	case "editor":
		if err := config.SetEditor(value); err != nil {
			ui.ShowError("Failed to set editor", err)
			return
		}
		ui.ShowSuccess("editor set to: %s", value)
	default:
		ui.ShowError(fmt.Sprintf("Unknown configuration key: %s", key), nil)
		fmt.Println("Available keys: default-behavior, skip-permissions, terminal, auto-update, editor")
	}
}

// RunConfigGet gets a configuration value (or all values if no key given).
func RunConfigGet(args []string) {
	if len(args) == 0 {
		cfg, err := config.LoadConfig()
		if err != nil {
			ui.ShowError("Error loading config", err)
			return
		}

		behavior := cfg.DefaultBehavior
		if behavior == "" {
			behavior = "current"
		}
		terminal := cfg.Terminal
		if terminal == "" {
			if runtime.GOOS == "windows" {
				terminal = "auto"
			} else {
				terminal = "terminal"
			}
		}
		autoUpdate := cfg.AutoUpdate
		if autoUpdate == "" {
			autoUpdate = "notify"
		}

		fmt.Println("Current configuration:")
		fmt.Printf("  default-behavior: %s\n", behavior)
		fmt.Printf("  skip-permissions: %v\n", cfg.SkipPermissions)
		fmt.Printf("  terminal: %s\n", terminal)
		fmt.Printf("  auto-update: %s\n", autoUpdate)
		editor := cfg.Editor
		if editor == "" {
			editor = "(auto-detect)"
		}
		fmt.Printf("  editor: %s\n", editor)
		fmt.Printf("  default: %s\n", cfg.Default)
		fmt.Printf("  projects: %d configured\n", len(cfg.Projects))
		return
	}

	key := args[0]
	switch key {
	case "default-behavior", "defaultBehavior":
		RunDefaultBehaviorGet()
	case "skip-permissions", "skipPermissions":
		RunSkipPermissionsGet()
	case "terminal":
		RunTerminalGet()
	case "auto-update", "autoUpdate":
		fmt.Printf("auto-update: %s\n", config.GetAutoUpdate())
	case "editor":
		editor := config.GetEditor()
		if editor == "" {
			fmt.Println("editor: (auto-detect)")
		} else {
			fmt.Printf("editor: %s\n", editor)
		}
	default:
		ui.ShowError(fmt.Sprintf("Unknown configuration key: %s", key), nil)
		fmt.Println("Available keys: default-behavior, skip-permissions, terminal, auto-update, editor")
	}
}

// RunDefaultBehaviorSet sets the default startup directory behavior.
func RunDefaultBehaviorSet(behavior string) {
	if behavior != "current" && behavior != "last" && behavior != "home" {
		ui.ShowError("Invalid behavior. Must be 'current', 'last', or 'home'", nil)
		fmt.Println()
		ui.ShowInfo("Available behaviors:")
		ui.ShowInfo("  current - Use current working directory")
		ui.ShowInfo("  last    - Use last used directory")
		ui.ShowInfo("  home    - Use home directory")
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	oldBehavior := cfg.DefaultBehavior
	if oldBehavior == "" {
		oldBehavior = "current"
	}

	cfg.DefaultBehavior = behavior

	if err := config.SaveConfig(cfg); err != nil {
		ui.ShowError("Error saving config", err)
		return
	}

	ui.ShowSuccess("Default behavior set to: %s", behavior)
	fmt.Println()
	ui.ShowInfo("This will affect where Claude starts when you run 'codes' without arguments.")
	ui.ShowInfo("Previous behavior: %s", oldBehavior)
	ui.ShowInfo("New behavior: %s", behavior)

	fmt.Println()
	ui.ShowInfo("Examples:")
	ui.ShowInfo("  codes                    - Start Claude with %s directory", behavior)
	ui.ShowInfo("  codes start project-name - Start Claude in specific project")
	ui.ShowInfo("  codes start /path/to/dir - Start Claude in specific directory")
}

// RunDefaultBehaviorGet shows the current default behavior setting.
func RunDefaultBehaviorGet() {
	currentBehavior := config.GetDefaultBehavior()

	fmt.Println("Current default behavior:")
	ui.ShowInfo("  %s", currentBehavior)

	fmt.Println()
	ui.ShowInfo("Description:")
	switch currentBehavior {
	case "current":
		ui.ShowInfo("  Claude will start in the current working directory")
	case "last":
		ui.ShowInfo("  Claude will start in the last used directory")
	case "home":
		ui.ShowInfo("  Claude will start in your home directory")
	}

	fmt.Println()
	ui.ShowInfo("To change this setting:")
	ui.ShowInfo("  codes config set default-behavior <current|last|home>")
}

// RunDefaultBehaviorReset resets the default behavior to "current".
func RunDefaultBehaviorReset() {
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	oldBehavior := cfg.DefaultBehavior
	if oldBehavior == "" {
		oldBehavior = "current"
	}

	cfg.DefaultBehavior = ""

	if err := config.SaveConfig(cfg); err != nil {
		ui.ShowError("Error saving config", err)
		return
	}

	ui.ShowSuccess("Default behavior reset to: current")
	fmt.Println()
	ui.ShowInfo("Previous behavior: %s", oldBehavior)
	ui.ShowInfo("New behavior: current (default)")
	ui.ShowInfo("Claude will now start in the current working directory by default.")
}

// RunSkipPermissionsSet sets the global skipPermissions flag.
func RunSkipPermissionsSet(skip bool) {
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	oldValue := cfg.SkipPermissions
	cfg.SkipPermissions = skip

	if err := config.SaveConfig(cfg); err != nil {
		ui.ShowError("Error saving config", err)
		return
	}

	status := "enabled"
	if !skip {
		status = "disabled"
	}
	ui.ShowSuccess("Global skipPermissions %s", status)

	fmt.Println()
	ui.ShowInfo("Previous setting: %v", oldValue)
	ui.ShowInfo("New setting: %v", skip)

	if skip {
		ui.ShowInfo("Claude will now run with --dangerously-skip-permissions for all configurations that don't have their own setting.")
	} else {
		ui.ShowInfo("Claude will run without --dangerously-skip-permissions unless a specific configuration has it enabled.")
	}
}

// RunSkipPermissionsGet shows the current global skipPermissions setting.
func RunSkipPermissionsGet() {
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	fmt.Printf("Global skipPermissions: %v\n", cfg.SkipPermissions)

	if cfg.SkipPermissions {
		ui.ShowInfo("Claude will run with --dangerously-skip-permissions for all configurations that don't have their own setting.")
	} else {
		ui.ShowInfo("Claude will run without --dangerously-skip-permissions unless a specific configuration has it enabled.")
	}

	fmt.Println()
	ui.ShowInfo("Individual configuration settings override this global setting.")
	ui.ShowInfo("Use 'codes config get' to see all configurations and their skipPermissions settings.")
}

// RunSkipPermissionsReset resets the global skipPermissions to false.
func RunSkipPermissionsReset() {
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	oldValue := cfg.SkipPermissions
	cfg.SkipPermissions = false

	if err := config.SaveConfig(cfg); err != nil {
		ui.ShowError("Error saving config", err)
		return
	}

	ui.ShowSuccess("Global skipPermissions reset to: false")
	fmt.Println()
	ui.ShowInfo("Previous setting: %v", oldValue)
	ui.ShowInfo("New setting: false (default)")
	ui.ShowInfo("Claude will now run without --dangerously-skip-permissions unless a specific configuration has it enabled.")
}

// RunConfigReset resets one or all configuration keys.
func RunConfigReset(args []string) {
	if len(args) == 0 {
		RunDefaultBehaviorReset()
		RunSkipPermissionsReset()
		RunTerminalReset()
		if err := config.SetAutoUpdate(""); err != nil {
			ui.ShowWarning("Failed to reset auto-update: %v", err)
		} else {
			ui.ShowSuccess("auto-update reset to default (notify)")
		}
		if err := config.SetEditor(""); err != nil {
			ui.ShowWarning("Failed to reset editor: %v", err)
		} else {
			ui.ShowSuccess("editor reset to default (auto-detect)")
		}
		return
	}

	key := args[0]
	switch key {
	case "default-behavior", "defaultBehavior":
		RunDefaultBehaviorReset()
	case "skip-permissions", "skipPermissions":
		RunSkipPermissionsReset()
	case "terminal":
		RunTerminalReset()
	case "auto-update", "autoUpdate":
		if err := config.SetAutoUpdate(""); err != nil {
			ui.ShowWarning("Failed to reset auto-update: %v", err)
		} else {
			ui.ShowSuccess("auto-update reset to default (notify)")
		}
	case "editor":
		if err := config.SetEditor(""); err != nil {
			ui.ShowWarning("Failed to reset editor: %v", err)
		} else {
			ui.ShowSuccess("editor reset to default (auto-detect)")
		}
	default:
		ui.ShowError(fmt.Sprintf("Unknown configuration key: %s", key), nil)
		fmt.Println("Available keys: default-behavior, skip-permissions, terminal, auto-update, editor")
	}
}

// RunConfigList lists available values for a configuration key.
func RunConfigList(args []string) {
	if len(args) == 0 {
		fmt.Println("Available configuration keys:")
		fmt.Println("  default-behavior  Startup directory behavior (current, last, home)")
		fmt.Println("  skip-permissions  Global --dangerously-skip-permissions (true, false)")
		fmt.Println("  terminal          Terminal emulator for sessions")
		fmt.Println("  auto-update       Auto-update check mode (notify, silent, off)")
		fmt.Println("  editor            Editor command for opening projects")
		fmt.Println()
		fmt.Println("Use 'codes config list <key>' to see available values for a key.")
		return
	}

	key := args[0]
	switch key {
	case "default-behavior", "defaultBehavior":
		fmt.Println("Available values for default-behavior:")
		fmt.Println("  current  Use current working directory (default)")
		fmt.Println("  last     Use last used directory")
		fmt.Println("  home     Use home directory")
	case "skip-permissions", "skipPermissions":
		fmt.Println("Available values for skip-permissions:")
		fmt.Println("  true     Enable --dangerously-skip-permissions globally")
		fmt.Println("  false    Disable --dangerously-skip-permissions globally (default)")
	case "terminal":
		RunTerminalList()
	case "auto-update", "autoUpdate":
		fmt.Println("Available values for auto-update:")
		fmt.Println("  notify   Show notification when new version is available (default)")
		fmt.Println("  silent   Download new version in background, apply on next launch")
		fmt.Println("  off      Disable automatic update checks")
	case "editor":
		fmt.Println("Available values for editor:")
		fmt.Println("  code     Visual Studio Code")
		fmt.Println("  cursor   Cursor")
		fmt.Println("  zed      Zed")
		fmt.Println("  subl     Sublime Text")
		fmt.Println("  vim      Vim")
		fmt.Println("  nvim     Neovim")
		fmt.Println("  <cmd>    Any command that accepts a path argument")
	default:
		ui.ShowError(fmt.Sprintf("Unknown configuration key: %s", key), nil)
		fmt.Println("Available keys: default-behavior, skip-permissions, terminal, auto-update, editor")
	}
}

// RunTerminalReset resets the terminal setting to the platform default.
func RunTerminalReset() {
	old := config.GetTerminal()

	if err := config.SetTerminal(""); err != nil {
		ui.ShowError("Error saving config", err)
		return
	}

	defaultTerminal := "terminal"
	if runtime.GOOS == "windows" {
		defaultTerminal = "auto"
	}

	ui.ShowSuccess("Terminal reset to: %s (platform default)", defaultTerminal)
	if old != "" {
		ui.ShowInfo("Previous: %s", old)
	}
}

// RunTerminalSet sets the terminal emulator preference.
func RunTerminalSet(terminal string) {
	old := config.GetTerminal()
	if old == "" {
		if runtime.GOOS == "windows" {
			old = "auto"
		} else {
			old = "terminal"
		}
	}

	if err := config.SetTerminal(terminal); err != nil {
		ui.ShowError("Error saving config", err)
		return
	}

	ui.ShowSuccess("Terminal set to: %s", terminal)
	fmt.Println()
	ui.ShowInfo("Previous: %s", old)
	ui.ShowInfo("New: %s", terminal)
	fmt.Println()

	switch terminal {
	case "terminal":
		ui.ShowInfo("Sessions will open in Terminal.app")
	case "iterm", "iterm2":
		ui.ShowInfo("Sessions will open in iTerm2")
	case "warp":
		ui.ShowInfo("Sessions will open in Warp")
	case "auto":
		ui.ShowInfo("Sessions will open in the best available terminal")
	case "wt":
		ui.ShowInfo("Sessions will open in Windows Terminal")
	case "powershell":
		ui.ShowInfo("Sessions will open in Windows PowerShell")
	case "pwsh":
		ui.ShowInfo("Sessions will open in PowerShell Core")
	case "cmd":
		ui.ShowInfo("Sessions will open in Command Prompt")
	default:
		ui.ShowInfo("Sessions will open with: %s", terminal)
	}
}

// RunTerminalGet shows the current terminal emulator setting.
func RunTerminalGet() {
	current := config.GetTerminal()
	if current == "" {
		current = "terminal"
	}

	fmt.Println("Current terminal emulator:")
	ui.ShowInfo("  %s", current)
	fmt.Println()
	ui.ShowInfo("To change: codes config set terminal <terminal>")
	ui.ShowInfo("To list options: codes config list terminal")
}

// RunTerminalList lists available terminal emulator options.
func RunTerminalList() {
	current := config.GetTerminal()

	fmt.Println("Available terminal emulators:")
	fmt.Println()

	var options []struct {
		name string
		desc string
	}

	if runtime.GOOS == "windows" {
		if current == "" {
			current = "auto"
		}
		options = []struct {
			name string
			desc string
		}{
			{"auto", "Auto-detect (Windows Terminal > PowerShell)"},
			{"wt", "Windows Terminal"},
			{"powershell", "Windows PowerShell"},
			{"pwsh", "PowerShell Core"},
			{"cmd", "Command Prompt"},
		}
	} else {
		if current == "" {
			current = "terminal"
		}
		options = []struct {
			name string
			desc string
		}{
			{"terminal", "macOS Terminal.app (default)"},
			{"iterm", "iTerm2"},
			{"warp", "Warp"},
		}
	}

	for _, opt := range options {
		marker := "  "
		if opt.name == current {
			marker = "► "
		}
		ui.ShowInfo("%s%-10s %s", marker, opt.name, opt.desc)
	}

	fmt.Println()
	ui.ShowInfo("You can also use any custom terminal command:")
	if runtime.GOOS == "windows" {
		ui.ShowInfo("  codes config set terminal wt")
		ui.ShowInfo("  codes config set terminal pwsh")
	} else {
		ui.ShowInfo("  codes config set terminal Alacritty")
		ui.ShowInfo("  codes config set terminal /usr/bin/xterm")
	}
}
