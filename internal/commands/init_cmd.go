package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"codes/internal/config"
	"codes/internal/ui"
)

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func RunInit(autoYes bool) {
	ui.ShowHeader("Codes CLI Setup")
	fmt.Println()

	allGood := true

	// 0. Check if Git is installed
	ui.ShowInfo("Checking Git installation...")
	if !checkGitAvailable(autoYes) {
		allGood = false
	}
	fmt.Println()

	// 1. Check PowerShell execution policy (Windows only, no-op on other platforms)
	if runtime.GOOS == "windows" {
		ui.ShowInfo("Checking PowerShell execution policy...")
		if !checkExecutionPolicy(autoYes) {
			allGood = false
		}
		fmt.Println()
	}

	// 2. Install binary to system PATH
	ui.ShowInfo("Installing codes CLI...")
	if path, _ := installBinary(); path == "" {
		allGood = false
	}
	fmt.Println()

	// 3. Install shell completion
	ui.ShowInfo("Setting up shell completion...")
	if !installShellCompletion() {
		allGood = false
	}
	fmt.Println()

	// 4. Check if Claude CLI is installed
	ui.ShowInfo("Checking Claude CLI installation...")
	if _, err := exec.LookPath("claude"); err != nil {
		ui.ShowError("Claude CLI not found", nil)
		ui.ShowWarning("  Run 'codes claude update' to install Claude CLI")
		allGood = false
	} else {
		ui.ShowSuccess("Claude CLI is installed")

		cmd := exec.Command("claude", "--version")
		output, err := cmd.Output()
		if err == nil {
			version := strings.TrimSpace(string(output))
			ui.ShowInfo("  Version: %s", version)
		}
	}
	fmt.Println()

	// 5. Check for existing environment variables
	ui.ShowInfo("Checking for existing Claude configuration...")
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	authToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")

	hasEnvConfig := false
	if baseURL != "" && authToken != "" {
		ui.ShowSuccess("Found existing configuration in environment variables")
		ui.ShowInfo("  ANTHROPIC_BASE_URL: %s", baseURL)
		ui.ShowInfo("  ANTHROPIC_AUTH_TOKEN: %s...", authToken[:minInt(10, len(authToken))])
		hasEnvConfig = true
	} else if baseURL != "" || authToken != "" {
		ui.ShowWarning("Incomplete environment configuration detected")
		if baseURL != "" {
			ui.ShowInfo("  ANTHROPIC_BASE_URL: %s", baseURL)
		}
		if authToken != "" {
			ui.ShowInfo("  ANTHROPIC_AUTH_TOKEN: configured")
		}
	}
	fmt.Println()

	// 6. Check if config file exists
	ui.ShowInfo("Checking configuration file...")
	configExists := false
	if _, err := os.Stat(config.ConfigPath); err == nil {
		configExists = true
	}

	// If env config exists but no codes config, offer to import
	if hasEnvConfig && !configExists {
		doImport := autoYes
		if !autoYes {
			fmt.Println()
			ui.ShowInfo("Would you like to import this configuration? (y/n): ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			doImport = response == "y" || response == "yes"
		}

		if doImport {
			name := "imported"
			if !autoYes {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Enter a name for this configuration (default: imported): ")
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				if input != "" {
					name = input
				}
			}

			ui.ShowLoading("Testing API connection...")
			testConfig := config.APIConfig{
				Name: name,
				Env:  make(map[string]string),
			}
			testConfig.Env["ANTHROPIC_BASE_URL"] = baseURL
			testConfig.Env["ANTHROPIC_AUTH_TOKEN"] = authToken

			var cfg config.Config
			cfg.Profiles = []config.APIConfig{testConfig}
			cfg.Default = name

			if config.TestAPIConfig(testConfig) {
				ui.ShowSuccess("API connection successful!")
				testConfig.Status = "active"
			} else {
				ui.ShowWarning("API connection failed, but configuration will be saved")
				testConfig.Status = "inactive"
			}

			cfg.Profiles[0] = testConfig

			if err := config.SaveConfig(&cfg); err != nil {
				ui.ShowError("Failed to save configuration", err)
				allGood = false
			} else {
				ui.ShowSuccess("Configuration imported successfully!")
				ui.ShowInfo("  Name: %s", name)
				ui.ShowInfo("  Status: %s", testConfig.Status)
				configExists = true
			}
			fmt.Println()
		}
	}

	// Continue with normal config check
	if !configExists {
		ui.ShowError("Configuration file not found", nil)
		ui.ShowInfo("  Expected location: %s", config.ConfigPath)
		ui.ShowWarning("  Run 'codes profile add' to create your first configuration")
		allGood = false
	} else {
		ui.ShowSuccess("Configuration file exists")
		ui.ShowInfo("  Location: %s", config.ConfigPath)

		// 7. Validate configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			ui.ShowError("Failed to load configuration", err)
			ui.ShowWarning("  Your config file may be corrupted")
			allGood = false
		} else {
			if len(cfg.Profiles) == 0 {
				ui.ShowWarning("✗ No configurations found in config file")
				ui.ShowWarning("  Run 'codes profile add' to add a configuration")
				allGood = false
			} else {
				ui.ShowSuccess("Found %d configuration(s)", len(cfg.Profiles))

				fmt.Println()
				ui.ShowInfo("Configurations:")
				for i, c := range cfg.Profiles {
					isDefault := ""
					if c.Name == cfg.Default {
						isDefault = " (default)"
					}

					statusIcon := "?"
					statusText := "unknown"
					if c.Status == "active" {
						statusIcon = "✓"
						statusText = "active"
					} else if c.Status == "inactive" {
						statusIcon = "✗"
						statusText = "inactive"
					}

					permissionsText := "default"
					if config.ShouldSkipPermissions(&c) {
						permissionsText = "skip permissions"
					} else if c.SkipPermissions != nil && !*c.SkipPermissions {
						permissionsText = "use permissions"
					}

					apiURL := "unknown"
					if baseURL, exists := c.Env["ANTHROPIC_BASE_URL"]; exists {
						apiURL = baseURL
					}

					fmt.Printf("  %d. %s %s%s - %s [%s, %s]\n",
						i+1, statusIcon, c.Name, isDefault, apiURL, statusText, permissionsText)

					if len(c.Env) > 0 {
						fmt.Printf("      Environment Variables (%d):\n", len(c.Env))
						for envKey, envValue := range c.Env {
							displayValue := envValue
							if strings.Contains(strings.ToUpper(envKey), "TOKEN") ||
								strings.Contains(strings.ToUpper(envKey), "KEY") ||
								strings.Contains(strings.ToUpper(envKey), "SECRET") {
								if len(envValue) > 8 {
									displayValue = envValue[:4] + "..." + envValue[len(envValue)-4:]
								}
							}
							fmt.Printf("        %s: %s\n", envKey, displayValue)
						}
					}
				}

				if cfg.Default != "" {
					fmt.Println()
					ui.ShowInfo("Testing default configuration '%s'...", cfg.Default)

					var defaultConfig *config.APIConfig
					for i := range cfg.Profiles {
						if cfg.Profiles[i].Name == cfg.Default {
							defaultConfig = &cfg.Profiles[i]
							break
						}
					}

					if defaultConfig != nil {
						if config.TestAPIConfig(*defaultConfig) {
							ui.ShowSuccess("Default configuration is working")
						} else {
							ui.ShowWarning("✗ Default configuration test failed")
							ui.ShowWarning("  API may be unreachable or credentials may be invalid")
							ui.ShowInfo("  Run 'codes profile add' to add a new configuration")
							allGood = false
						}
					} else {
						ui.ShowWarning("✗ Default configuration '%s' not found", cfg.Default)
						ui.ShowWarning("  Run 'codes profile select' to choose a valid configuration")
						allGood = false
					}
				}
			}
		}
	}

	fmt.Println()
	ui.ShowInfo("─────────────────────────────────")
	fmt.Println()

	if allGood {
		ui.ShowSuccess("All checks passed! You're ready to use codes.")
		fmt.Println()
		ui.ShowInfo("Quick commands:")
		ui.ShowInfo("  codes          - Run Claude with current configuration")
		ui.ShowInfo("  codes pf select - Switch between configurations")
		ui.ShowInfo("  codes pf add    - Add a new configuration")
	} else {
		ui.ShowWarning("Some checks failed. Please review the messages above.")
		fmt.Println()
		ui.ShowInfo("Suggested actions:")
		if _, err := exec.LookPath("git"); err != nil {
			ui.ShowInfo("  1. Install Git")
		}
		if _, err := exec.LookPath("claude"); err != nil {
			ui.ShowInfo("  2. Install Claude CLI: codes claude update")
		}
		if _, err := os.Stat(config.ConfigPath); err != nil {
			ui.ShowInfo("  3. Add a configuration: codes profile add")
		}
	}
}
