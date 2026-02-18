package commands

import (
	"fmt"
	"time"

	"codes/internal/config"
	"codes/internal/ui"
)

// RunTest tests API configurations.
func RunTest(args []string) {
	ui.ShowHeader("API Configuration Test")
	fmt.Println()

	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	if len(cfg.Profiles) == 0 {
		ui.ShowError("No configurations found", nil)
		ui.ShowInfo("Run 'codes profile add' to add a configuration first")
		return
	}

	if len(args) > 0 && args[0] != "" {
		configName := args[0]
		var targetConfig *config.APIConfig
		for i := range cfg.Profiles {
			if cfg.Profiles[i].Name == configName {
				targetConfig = &cfg.Profiles[i]
				break
			}
		}

		if targetConfig == nil {
			ui.ShowError("Configuration '%s' not found", fmt.Errorf("config not found"))
			return
		}

		ui.ShowInfo("Testing configuration: %s", configName)
		testSingleConfiguration(targetConfig)
	} else {
		ui.ShowInfo("Testing all %d configurations...", len(cfg.Profiles))
		testAllConfigurations(cfg.Profiles)
	}
}

// testSingleConfiguration tests a single API configuration.
func testSingleConfiguration(apiConfig *config.APIConfig) {
	fmt.Println()

	envVars := config.GetEnvironmentVars(apiConfig)
	model := envVars["ANTHROPIC_MODEL"]
	if model == "" {
		model = envVars["ANTHROPIC_DEFAULT_HAIKU_MODEL"]
		if model == "" {
			model = "claude-3-haiku-20240307"
		}
	}

	ui.ShowInfo("Model: %s", model)
	ui.ShowInfo("API: %s", envVars["ANTHROPIC_BASE_URL"])

	ui.ShowLoading("Testing API connection...")
	start := time.Now()
	success := config.TestAPIConfig(*apiConfig)
	latency := time.Since(start)
	if success {
		ui.ShowSuccess("API connection successful! (Latency: %dms)", latency.Milliseconds())
		apiConfig.Status = "active"
	} else {
		ui.ShowError("API connection failed", nil)
		apiConfig.Status = "inactive"
		ui.ShowWarning("Check your configuration and network connectivity")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config for update", err)
		return
	}

	for i := range cfg.Profiles {
		if cfg.Profiles[i].Name == apiConfig.Name {
			cfg.Profiles[i].Status = apiConfig.Status
			break
		}
	}

	if err := config.SaveConfig(cfg); err != nil {
		ui.ShowError("Failed to save config status", err)
	}
}

// testAllConfigurations tests all API configurations.
func testAllConfigurations(configs []config.APIConfig) {
	results := make(map[string]bool)
	statuses := make(map[string]string)
	successCount := 0

	fmt.Println()
	for i := range configs {
		fmt.Printf("Testing %s...", configs[i].Name)

		envVars := config.GetEnvironmentVars(&configs[i])
		model := envVars["ANTHROPIC_MODEL"]
		if model == "" {
			model = envVars["ANTHROPIC_DEFAULT_HAIKU_MODEL"]
			if model == "" {
				model = "claude-3-haiku-20240307"
			}
		}

		start := time.Now()
		success := config.TestAPIConfig(configs[i])
		latency := time.Since(start)
		results[configs[i].Name] = success

		if success {
			fmt.Printf(" ✓ (Model: %s, Latency: %dms)\n", model, latency.Milliseconds())
			statuses[configs[i].Name] = "active"
			successCount++
		} else {
			fmt.Printf(" ✗ (Model: %s, Latency: %dms)\n", model, latency.Milliseconds())
			statuses[configs[i].Name] = "inactive"
		}
	}

	fmt.Println()
	ui.ShowHeader("Test Results")
	fmt.Printf("Successfully tested: %d/%d\n", successCount, len(configs))

	if successCount == len(configs) {
		ui.ShowSuccess("All configurations are working!")
	} else if successCount == 0 {
		ui.ShowError("No configurations are working", nil)
		ui.ShowInfo("Check your configurations and network connectivity")
	} else {
		ui.ShowWarning("Some configurations failed")
		ui.ShowInfo("Use 'codes profile test <config-name>' to test individual configurations")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config for update", err)
		return
	}

	updated := false
	for i := range cfg.Profiles {
		if newStatus, ok := statuses[cfg.Profiles[i].Name]; ok {
			if cfg.Profiles[i].Status != newStatus {
				cfg.Profiles[i].Status = newStatus
				updated = true
			}
		}
	}

	if updated {
		if err := config.SaveConfig(cfg); err != nil {
			ui.ShowError("Failed to save config status", err)
		}
	}

	// suppress unused variable warning
	_ = results
}

// RunProfileList lists all profiles and their status.
func RunProfileList() {
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	if len(cfg.Profiles) == 0 {
		ui.ShowInfo("No profiles configured yet")
		ui.ShowInfo("Add a profile with: codes profile add")
		return
	}

	fmt.Println()
	ui.ShowHeader("API Profiles")
	fmt.Println()

	for i, c := range cfg.Profiles {
		isDefault := ""
		if c.Name == cfg.Default {
			isDefault = " (default)"
		}

		apiURL := c.Env["ANTHROPIC_BASE_URL"]
		if apiURL == "" {
			apiURL = "unknown"
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

		ui.ShowInfo("%d. %s %s%s - %s [%s]", i+1, statusIcon, c.Name, isDefault, apiURL, statusText)
	}

	fmt.Println()
}

// RunProfileRemove removes a named profile.
func RunProfileRemove(name string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	found := -1
	for i, c := range cfg.Profiles {
		if c.Name == name {
			found = i
			break
		}
	}

	if found == -1 {
		ui.ShowError(fmt.Sprintf("Profile '%s' not found", name), nil)
		return
	}

	cfg.Profiles = append(cfg.Profiles[:found], cfg.Profiles[found+1:]...)

	if cfg.Default == name {
		if len(cfg.Profiles) > 0 {
			cfg.Default = cfg.Profiles[0].Name
			ui.ShowInfo("Default profile switched to: %s", cfg.Default)
		} else {
			cfg.Default = ""
		}
	}

	if err := config.SaveConfig(cfg); err != nil {
		ui.ShowError("Failed to save config", err)
		return
	}

	ui.ShowSuccess("Profile '%s' removed successfully!", name)
}
