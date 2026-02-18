package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"codes/internal/config"
	"codes/internal/ui"
)

func RunAdd() {
	ui.ShowHeader("Add New Claude Configuration")

	var configData config.Config
	if _, err := os.Stat(config.ConfigPath); err == nil {
		cfg, err := config.LoadConfig()
		if err != nil {
			ui.ShowError("Error loading existing config", err)
			return
		}
		configData = *cfg
	} else {
		configData.Profiles = []config.APIConfig{}
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter configuration name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		ui.ShowError("Configuration name cannot be empty", nil)
		return
	}

	for _, c := range configData.Profiles {
		if c.Name == name {
			ui.ShowError("Configuration '%s' already exists", fmt.Errorf("name '%s' already exists", name))
			return
		}
	}

	newConfig := config.APIConfig{
		Name: name,
		Env:  make(map[string]string),
	}

	defaultVars := config.GetDefaultEnvironmentVars()

	fmt.Println("\nBasic Configuration:")
	ui.ShowInfo("Enter values for required environment variables.")

	fmt.Print("Enter ANTHROPIC_BASE_URL (required): ")
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		ui.ShowError("Base URL cannot be empty", nil)
		return
	}
	newConfig.Env["ANTHROPIC_BASE_URL"] = baseURL

	fmt.Print("Enter ANTHROPIC_AUTH_TOKEN (required): ")
	authToken, _ := reader.ReadString('\n')
	authToken = strings.TrimSpace(authToken)
	if authToken == "" {
		ui.ShowError("Authentication token cannot be empty", nil)
		return
	}
	newConfig.Env["ANTHROPIC_AUTH_TOKEN"] = authToken

	fmt.Println("\nOptional Configuration:")
	ui.ShowInfo("The following environment variables are optional. Press Enter to skip.")

	modelVars := []string{
		"ANTHROPIC_MODEL",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL",
		"ANTHROPIC_DEFAULT_OPUS_MODEL",
		"ANTHROPIC_DEFAULT_SONNET_MODEL",
	}

	otherVars := make(map[string]string)
	for envVar, description := range defaultVars {
		if _, exists := newConfig.Env[envVar]; exists {
			continue
		}
		isModelVar := false
		for _, mv := range modelVars {
			if envVar == mv {
				isModelVar = true
				break
			}
		}
		if !isModelVar {
			otherVars[envVar] = description
		}
	}

	fmt.Println("\nModel Configuration:")
	ui.ShowInfo("These are model-specific variables. You can enter values or type 'skip' to use defaults.")

	for _, envVar := range modelVars {
		if _, exists := newConfig.Env[envVar]; exists {
			continue
		}

		description := defaultVars[envVar]
		fmt.Printf("Enter %s (%s) [skip]: ", envVar, description)
		value, _ := reader.ReadString('\n')
		value = strings.TrimSpace(value)

		if value == "skip" {
			ui.ShowInfo("Skipping %s", envVar)
		} else if value != "" {
			newConfig.Env[envVar] = value
		}
	}

	fmt.Println("\nOther Optional Configuration:")
	ui.ShowInfo("The following environment variables are optional. Press Enter to skip.")

	for envVar, description := range otherVars {
		fmt.Printf("Enter %s (%s): ", envVar, description)
		value, _ := reader.ReadString('\n')
		value = strings.TrimSpace(value)
		if value != "" {
			newConfig.Env[envVar] = value
		}
	}

	fmt.Print("\nWould you like to add any additional environment variables? (y/n): ")
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "y" || response == "yes" {
		fmt.Println("Enter environment variables in the format: VARIABLE_NAME=value")
		fmt.Println("Enter an empty line to finish")

		for {
			fmt.Print("> ")
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)

			if line == "" {
				break
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				varName := strings.TrimSpace(parts[0])
				varValue := strings.TrimSpace(parts[1])
				if varName != "" {
					newConfig.Env[varName] = varValue
				}
			} else {
				ui.ShowWarning("Invalid format. Use VARIABLE_NAME=value")
			}
		}
	}

	fmt.Print("Use --dangerously-skip-permissions? (y/n) [default: n]: ")
	skipResp, _ := reader.ReadString('\n')
	skipResp = strings.TrimSpace(strings.ToLower(skipResp))
	if skipResp == "y" {
		skipPermissions := true
		newConfig.SkipPermissions = &skipPermissions
	}

	ui.ShowLoading("Testing API connection")
	if config.TestAPIConfig(newConfig) {
		ui.ShowSuccess("API connection successful!")
		newConfig.Status = "active"
	} else {
		ui.ShowWarning("API connection failed. Configuration added but marked as inactive")
		newConfig.Status = "inactive"
	}

	configData.Profiles = append(configData.Profiles, newConfig)

	if len(configData.Profiles) == 1 {
		configData.Default = name
		ui.ShowInfo("Set '%s' as default configuration", name)
	}

	if err := config.SaveConfig(&configData); err != nil {
		ui.ShowError("Failed to save config", err)
		return
	}

	ui.ShowSuccess("Configuration '%s' added successfully!", name)
	ui.ShowInfo("API: %s", baseURL)
	ui.ShowInfo("Environment variables: %d", len(newConfig.Env))

	if newConfig.Status == "active" {
		ui.ShowInfo("Status: Active")
	} else {
		ui.ShowWarning("Status: Inactive (API test failed)")
	}

	if newConfig.SkipPermissions != nil {
		if *newConfig.SkipPermissions {
			ui.ShowInfo("Permissions: Skip --dangerously-skip-permissions")
		} else {
			ui.ShowInfo("Permissions: Use default (no --dangerously-skip-permissions)")
		}
	}
}
