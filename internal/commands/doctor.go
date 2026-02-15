package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"codes/internal/agent"
	"codes/internal/config"
	"codes/internal/ui"
)

// RunDoctor performs diagnostic checks on the system.
func RunDoctor() {
	ui.ShowHeader("Running System Diagnostics")
	fmt.Println()

	passCount := 0
	failCount := 0
	warnCount := 0

	// 1. Check Claude CLI installation
	fmt.Println("1. Checking Claude CLI...")
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		ui.ShowError("Claude CLI not found in PATH", err)
		ui.ShowInfo("Install with: npm install -g @anthropic-ai/claude")
		failCount++
	} else {
		ui.ShowSuccess("Claude CLI found: %s", claudePath)

		// Check version
		cmd := exec.Command("claude", "--version")
		output, err := cmd.Output()
		if err == nil {
			version := strings.TrimSpace(string(output))
			ui.ShowInfo("Version: %s", version)
		}
		passCount++
	}
	fmt.Println()

	// 2. Check config file
	fmt.Println("2. Checking configuration file...")
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Failed to load config", err)
		ui.ShowInfo("Run 'codes init' to create config")
		failCount++
	} else {
		ui.ShowSuccess("Config loaded: %s", config.ConfigPath)

		// Check profiles
		if len(cfg.Profiles) == 0 {
			ui.ShowWarning("No API profiles configured")
			ui.ShowInfo("Add a profile with: codes profile add")
			warnCount++
		} else {
			ui.ShowSuccess("%d profile(s) configured", len(cfg.Profiles))

			// Check default profile
			if cfg.Default == "" {
				ui.ShowWarning("No default profile set")
				warnCount++
			} else {
				ui.ShowSuccess("Default profile: %s", cfg.Default)
			}
		}
		passCount++
	}
	fmt.Println()

	// 3. Check API connectivity
	fmt.Println("3. Checking API connectivity...")
	if cfg == nil || len(cfg.Profiles) == 0 {
		ui.ShowWarning("Skipping API test (no profiles configured)")
		warnCount++
	} else {
		var defaultProfile *config.APIConfig
		for i := range cfg.Profiles {
			if cfg.Profiles[i].Name == cfg.Default {
				defaultProfile = &cfg.Profiles[i]
				break
			}
		}

		if defaultProfile == nil && len(cfg.Profiles) > 0 {
			defaultProfile = &cfg.Profiles[0]
		}

		if defaultProfile != nil {
			ui.ShowInfo("Testing profile: %s", defaultProfile.Name)
			if config.TestAPIConfig(*defaultProfile) {
				ui.ShowSuccess("API connection successful")
				passCount++
			} else {
				ui.ShowError("API test failed", nil)
				failCount++
			}
		}
	}
	fmt.Println()

	// 4. Check file permissions
	fmt.Println("4. Checking file permissions...")

	// Check config directory
	configDir := filepath.Dir(config.ConfigPath)
	if ui.CanWriteTo(configDir) {
		ui.ShowSuccess("Config directory writable: %s", configDir)
		passCount++
	} else {
		ui.ShowError("Config directory not writable", nil)
		ui.ShowInfo("Directory: %s", configDir)
		failCount++
	}

	// Check home directory for session data
	homeDir, err := os.UserHomeDir()
	if err == nil {
		sessionDir := filepath.Join(homeDir, ".claude")
		if _, err := os.Stat(sessionDir); err == nil {
			if ui.CanWriteTo(sessionDir) {
				ui.ShowSuccess("Session directory writable: %s", sessionDir)
				passCount++
			} else {
				ui.ShowWarning("Session directory not writable: %s", sessionDir)
				warnCount++
			}
		} else {
			ui.ShowInfo("Session directory will be created on first use: %s", sessionDir)
			passCount++
		}
	}
	fmt.Println()

	// 5. Check agent daemon status
	fmt.Println("5. Checking agent daemons...")
	teams, err := agent.ListTeams()
	if err != nil {
		ui.ShowWarning("Failed to list teams: %v", err)
		warnCount++
	} else if len(teams) == 0 {
		ui.ShowInfo("No agent teams configured")
		passCount++
	} else {
		totalAgents := 0
		runningAgents := 0

		for _, teamName := range teams {
			teamCfg, err := agent.GetTeam(teamName)
			if err != nil {
				ui.ShowWarning("Failed to load team %s: %v", teamName, err)
				warnCount++
				continue
			}

			for _, member := range teamCfg.Members {
				totalAgents++
				if agent.IsAgentAlive(teamName, member.Name) {
					runningAgents++
				}
			}
		}

		if totalAgents == 0 {
			ui.ShowInfo("No agents configured")
			passCount++
		} else {
			ui.ShowSuccess("%d/%d agents running across %d team(s)", runningAgents, totalAgents, len(teams))
			if runningAgents < totalAgents {
				ui.ShowInfo("Use 'codes agent start-all <team>' to start stopped agents")
			}
			passCount++
		}
	}
	fmt.Println()

	// 6. Check disk space
	fmt.Println("6. Checking disk space...")
	homeDir, err = os.UserHomeDir()
	if err != nil {
		ui.ShowWarning("Failed to get home directory: %v", err)
		warnCount++
	} else {
		claudeDir := filepath.Join(homeDir, ".claude")
		codesDir := filepath.Join(homeDir, ".codes")

		// Get directory sizes
		claudeSize, err := getDirSize(claudeDir)
		if err == nil {
			ui.ShowInfo("Claude session data: %s (%s)", claudeDir, formatBytes(claudeSize))
		}

		codesSize, err := getDirSize(codesDir)
		if err == nil {
			ui.ShowInfo("Codes data: %s (%s)", codesDir, formatBytes(codesSize))
		}

		// Get available disk space
		var stat syscall.Statfs_t
		err = syscall.Statfs(homeDir, &stat)
		if err == nil {
			available := stat.Bavail * uint64(stat.Bsize)
			total := stat.Blocks * uint64(stat.Bsize)
			used := total - available
			usedPercent := float64(used) / float64(total) * 100

			ui.ShowInfo("Disk usage: %.1f%% (%s / %s available)",
				usedPercent, formatBytes(available), formatBytes(total))

			if usedPercent > 90 {
				ui.ShowWarning("Disk usage high (%.1f%%)", usedPercent)
				warnCount++
			} else if available < 1*1024*1024*1024 { // < 1GB
				ui.ShowWarning("Low disk space: %s available", formatBytes(available))
				warnCount++
			} else {
				ui.ShowSuccess("Sufficient disk space available")
				passCount++
			}
		} else {
			ui.ShowWarning("Failed to check disk space: %v", err)
			warnCount++
		}
	}
	fmt.Println()

	// Summary
	ui.ShowHeader("Diagnostic Summary")
	fmt.Printf("  ✓ Passed: %d\n", passCount)
	if warnCount > 0 {
		fmt.Printf("  ! Warnings: %d\n", warnCount)
	}
	if failCount > 0 {
		fmt.Printf("  ✗ Failed: %d\n", failCount)
	}
	fmt.Println()

	if failCount > 0 {
		ui.ShowError("System has critical issues", nil)
		os.Exit(1)
	} else if warnCount > 0 {
		ui.ShowWarning("System has non-critical warnings")
	} else {
		ui.ShowSuccess("All checks passed!")
	}
}

// getDirSize calculates the total size of a directory recursively.
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't read
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// formatBytes formats a byte count in human-readable format.
func formatBytes(bytes interface{}) string {
	var b int64
	switch v := bytes.(type) {
	case int64:
		b = v
	case uint64:
		b = int64(v)
	default:
		return "0 B"
	}

	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
