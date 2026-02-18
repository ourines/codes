package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"codes/internal/config"
	"codes/internal/ui"
)

func RunSelect() {
	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	fmt.Println()
	ui.ShowHeader("Available Claude Profiles")
	fmt.Println()

	for i, c := range cfg.Profiles {
		apiURL := c.Env["ANTHROPIC_BASE_URL"]
		if apiURL == "" {
			apiURL = "unknown"
		}

		if c.Name == cfg.Default {
			if c.Status == "active" {
				ui.ShowCurrentConfig(i+1, c.Name, apiURL)
				ui.ShowInfo("     Status: Active")
			} else if c.Status == "inactive" {
				ui.ShowCurrentConfig(i+1, c.Name, apiURL)
				ui.ShowWarning("     Status: Inactive")
			} else {
				ui.ShowCurrentConfig(i+1, c.Name, apiURL)
			}
		} else {
			if c.Status == "active" {
				ui.ShowConfigOption(i+1, c.Name, apiURL)
				ui.ShowInfo("     Status: Active")
			} else if c.Status == "inactive" {
				ui.ShowConfigOption(i+1, c.Name, apiURL)
				ui.ShowWarning("     Status: Inactive")
			} else {
				ui.ShowConfigOption(i+1, c.Name, apiURL)
			}
		}
	}

	fmt.Println()
	fmt.Println("Select configuration (or press Enter to start with current):")
	fmt.Print("Choice: ")
	reader := bufio.NewReader(os.Stdin)
	selection, _ := reader.ReadString('\n')
	selection = strings.TrimSpace(selection)

	if selection == "" {
		ui.ShowSuccess("Starting with current configuration...")
		RunClaudeWithConfig([]string{})
		return
	}

	if selectedIdx, err := strconv.Atoi(selection); err == nil && selectedIdx >= 1 && selectedIdx <= len(cfg.Profiles) {
		selectedConfig := cfg.Profiles[selectedIdx-1]
		cfg.Default = selectedConfig.Name

		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			ui.ShowError("Failed to save config", err)
			return
		}

		ui.ShowSuccess("Selected: %s", selectedConfig.Name)
		apiURL := selectedConfig.Env["ANTHROPIC_BASE_URL"]
		if apiURL == "" {
			apiURL = "unknown"
		}
		ui.ShowInfo("API: %s", apiURL)

		RunClaudeWithConfig([]string{})
	} else {
		ui.ShowWarning("Invalid selection, starting with current config...")
		RunClaudeWithConfig([]string{})
	}
}

func RunClaudeWithConfig(args []string) {
	checkForUpdates()

	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		os.Exit(1)
	}

	var selectedConfig config.APIConfig
	for _, c := range cfg.Profiles {
		if c.Name == cfg.Default {
			selectedConfig = c
			break
		}
	}

	config.SetEnvironmentVars(&selectedConfig)

	apiURL := selectedConfig.Env["ANTHROPIC_BASE_URL"]
	if apiURL == "" {
		apiURL = "unknown"
	}

	ui.ShowInfo("Using configuration: %s (%s)", selectedConfig.Name, apiURL)

	var claudeArgs []string
	if config.ShouldSkipPermissionsWithConfig(&selectedConfig, cfg) {
		claudeArgs = []string{"--dangerously-skip-permissions"}
	}

	if len(args) > 0 {
		claudeArgs = append(claudeArgs, args...)
	}

	cmd := exec.Command("claude", claudeArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// runClaudeInDirectory runs Claude in the specified directory.
func runClaudeInDirectory(dir string) {
	checkForUpdates()

	cmd := config.BuildClaudeCmd(dir)

	ui.ShowInfo("Working directory: %s", dir)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
