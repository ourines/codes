package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"codes/internal/config"
	mcpserver "codes/internal/mcp"
	"codes/internal/output"
	"codes/internal/remote"
	"codes/internal/ui"
	"codes/internal/update"
)

// Version information, set via ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func RunVersion() {
	fmt.Printf("codes version %s (commit %s, built %s)\n", Version, Commit, Date)
}

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
		// 直接启动Claude
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

		// 立即启动Claude
		RunClaudeWithConfig([]string{})
	} else {
		ui.ShowWarning("Invalid selection, starting with current config...")
		RunClaudeWithConfig([]string{})
	}
}

func RunClaudeUpdate() {
	ui.ShowHeader("Claude Version Manager")
	ui.ShowLoading("Fetching available versions...")

	cmd := exec.Command("npm", "view", "@anthropic-ai/claude-code", "versions", "--json")
	output, err := cmd.Output()
	if err != nil {
		ui.ShowError("Failed to fetch Claude versions", nil)
		return
	}

	var versions []string
	if err := json.Unmarshal(output, &versions); err != nil {
		ui.ShowError("Failed to parse versions", nil)
		return
	}

	fmt.Println()
	ui.ShowInfo("Found %d available versions", len(versions))
	fmt.Println()

	// 显示最新20个版本
	ui.ShowInfo("Latest versions:")
	displayCount := 20
	if len(versions) < displayCount {
		displayCount = len(versions)
	}

	// 从最新版本开始显示（npm返回的是从旧到新）
	startIndex := len(versions) - displayCount
	for i := 0; i < displayCount; i++ {
		versionIndex := startIndex + i
		ui.ShowVersionItem(i+1, versions[versionIndex])
	}

	if len(versions) > displayCount {
		fmt.Println()
		ui.ShowInfo("(Showing %d most recent versions out of %d total)", displayCount, len(versions))
	}

	fmt.Println()
	fmt.Printf("Select version (1-%d, version number, or 'latest'): ", displayCount)
	reader := bufio.NewReader(os.Stdin)
	selection, _ := reader.ReadString('\n')
	selection = strings.TrimSpace(selection)

	// 检查是否为空
	if selection == "" {
		ui.ShowWarning("No selection made. Installing latest...")
		installClaude("latest")
		return
	}

	// 检查是否是 "latest"
	if selection == "latest" {
		ui.ShowLoading("Installing Claude latest...")
		installClaude("latest")
		return
	}

	// 尝试作为数字解析（从显示列表中选择）
	if selectedIdx, err := strconv.Atoi(selection); err == nil && selectedIdx >= 1 && selectedIdx <= displayCount {
		versionIndex := startIndex + selectedIdx - 1
		selectedVersion := versions[versionIndex]
		ui.ShowLoading("Installing Claude %s...", selectedVersion)
		installClaude(selectedVersion)
		return
	}

	// 作为自定义版本号处理
	ui.ShowLoading("Installing Claude %s...", selection)
	installClaude(selection)
}

func RunAdd() {
	ui.ShowHeader("Add New Claude Configuration")

	// 检查是否已存在配置文件，如果不存在则创建
	var configData config.Config
	if _, err := os.Stat(config.ConfigPath); err == nil {
		// 读取现有配置
		cfg, err := config.LoadConfig()
		if err != nil {
			ui.ShowError("Error loading existing config", err)
			return
		}
		configData = *cfg
	} else {
		// 创建新的配置
		configData.Profiles = []config.APIConfig{}
	}

	reader := bufio.NewReader(os.Stdin)

	// 获取配置名称
	fmt.Print("Enter configuration name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		ui.ShowError("Configuration name cannot be empty", nil)
		return
	}

	// 检查名称是否已存在
	for _, c := range configData.Profiles {
		if c.Name == name {
			ui.ShowError("Configuration '%s' already exists", fmt.Errorf("name '%s' already exists", name))
			return
		}
	}

	// 创建新的API配置
	newConfig := config.APIConfig{
		Name: name,
		Env:  make(map[string]string),
	}

	// 显示常用环境变量提示
	defaultVars := config.GetDefaultEnvironmentVars()

	// 基本必需环境变量
	fmt.Println("\nBasic Configuration:")
	ui.ShowInfo("Enter values for required environment variables.")

	// 获取ANTHROPIC_BASE_URL（必需）
	fmt.Print("Enter ANTHROPIC_BASE_URL (required): ")
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		ui.ShowError("Base URL cannot be empty", nil)
		return
	}
	newConfig.Env["ANTHROPIC_BASE_URL"] = baseURL

	// 获取认证令牌（必需）
	fmt.Print("Enter ANTHROPIC_AUTH_TOKEN (required): ")
	authToken, _ := reader.ReadString('\n')
	authToken = strings.TrimSpace(authToken)
	if authToken == "" {
		ui.ShowError("Authentication token cannot be empty", nil)
		return
	}
	newConfig.Env["ANTHROPIC_AUTH_TOKEN"] = authToken

	// 显示可选环境变量
	fmt.Println("\nOptional Configuration:")
	ui.ShowInfo("The following environment variables are optional. Press Enter to skip.")

	// 询问可选的环境变量
	modelVars := []string{
		"ANTHROPIC_MODEL",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL",
		"ANTHROPIC_DEFAULT_OPUS_MODEL",
		"ANTHROPIC_DEFAULT_SONNET_MODEL",
	}

	// 其他可选环境变量
	otherVars := make(map[string]string)
	for envVar, description := range defaultVars {
		// 跳过已设置的环境变量和模型变量
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

	// 首先询问模型相关的环境变量
	fmt.Println("\nModel Configuration:")
	ui.ShowInfo("These are model-specific variables. You can enter values or type 'skip' to use defaults.")

	for _, envVar := range modelVars {
		// 跳过已经设置的环境变量
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

	// 然后询问其他可选环境变量
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

	// 询问是否要设置其他环境变量
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

			// 解析 VARIABLE_NAME=value 格式
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

	// 询问是否跳过权限检查
	fmt.Print("Use --dangerously-skip-permissions? (y/n) [default: n]: ")
	skipResp, _ := reader.ReadString('\n')
	skipResp = strings.TrimSpace(strings.ToLower(skipResp))
	if skipResp == "y" {
		skipPermissions := true
		newConfig.SkipPermissions = &skipPermissions
	}

	// 测试API连接
	ui.ShowLoading("Testing API connection")
	if config.TestAPIConfig(newConfig) {
		ui.ShowSuccess("API connection successful!")
		newConfig.Status = "active"
	} else {
		ui.ShowWarning("API connection failed. Configuration added but marked as inactive")
		newConfig.Status = "inactive"
	}

	// 添加新配置
	configData.Profiles = append(configData.Profiles, newConfig)

	// 如果这是第一个配置，设置为默认
	if len(configData.Profiles) == 1 {
		configData.Default = name
		ui.ShowInfo("Set '%s' as default configuration", name)
	}

	// 保存配置
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

func RunClaudeWithConfig(args []string) {
	// 调用更新检查
	checkForUpdates()

	// Load and apply config
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		os.Exit(1)
	}

	// Find selected config
	var selectedConfig config.APIConfig
	for _, c := range cfg.Profiles {
		if c.Name == cfg.Default {
			selectedConfig = c
			break
		}
	}

	// Set environment variables
	config.SetEnvironmentVars(&selectedConfig)

	// Get API URL for display
	apiURL := selectedConfig.Env["ANTHROPIC_BASE_URL"]
	if apiURL == "" {
		apiURL = "unknown"
	}

	ui.ShowInfo("Using configuration: %s (%s)", selectedConfig.Name, apiURL)

	// Build claude command with or without --dangerously-skip-permissions
	var claudeArgs []string
	if config.ShouldSkipPermissionsWithConfig(&selectedConfig, cfg) {
		claudeArgs = []string{"--dangerously-skip-permissions"}
	}

	// Add user arguments
	if len(args) > 0 {
		claudeArgs = append(claudeArgs, args...)
	}

	cmd := exec.Command("claude", claudeArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func InstallClaude(version string) {
	installClaude(version)
}

func installClaude(version string) {
	cmd := exec.Command("npm", "install", "-g", fmt.Sprintf("@anthropic-ai/claude-code@%s", version))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		ui.ShowError("Installation failed", nil)
		os.Exit(1)
	}
	ui.ShowSuccess("Claude installed successfully!")
}

// installBinary copies the codes binary to a system PATH location.
// Returns the install path and whether it was newly installed.
func installBinary() (string, bool) {
	executablePath, err := os.Executable()
	if err != nil {
		ui.ShowError("Failed to get executable path", err)
		return "", false
	}

	var targetDir string
	var installPath string

	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				ui.ShowError("Failed to get home directory", err)
				return "", false
			}
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		targetDir = filepath.Join(localAppData, "codes")
		installPath = filepath.Join(targetDir, "codes.exe")
	default:
		if ui.CanWriteTo("/usr/local/bin") {
			targetDir = "/usr/local/bin"
			installPath = filepath.Join(targetDir, "codes")
		} else {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				ui.ShowError("Failed to get home directory", err)
				return "", false
			}
			targetDir = filepath.Join(homeDir, "bin")
			installPath = filepath.Join(targetDir, "codes")
		}
	}

	// Check if already installed at target and same binary
	executablePath, _ = filepath.EvalSymlinks(executablePath)
	targetResolved, _ := filepath.EvalSymlinks(installPath)
	if executablePath == targetResolved {
		ui.ShowSuccess("codes is already installed at %s", installPath)
		return installPath, false
	}

	ui.ShowInfo("Installing codes to: %s", installPath)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		ui.ShowError("Failed to create target directory", err)
		return "", false
	}

	src, err := os.Open(executablePath)
	if err != nil {
		ui.ShowError("Failed to read executable", err)
		return "", false
	}
	defer src.Close()

	dst, err := os.OpenFile(installPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		ui.ShowError("Failed to write to target location", err)
		return "", false
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		ui.ShowError("Failed to copy executable", err)
		return "", false
	}

	ui.ShowSuccess("codes installed to %s", installPath)

	if runtime.GOOS == "windows" {
		ensureInPath(targetDir)
	} else if targetDir != "/usr/local/bin" {
		ui.ShowWarning("  Make sure %s is in your PATH", targetDir)
	}

	return installPath, true
}

// installShellCompletion detects the user's shell and installs completion.
func installShellCompletion() bool {
	if runtime.GOOS == "windows" {
		return installPowerShellCompletion()
	}

	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		ui.ShowWarning("Could not detect shell, skipping completion setup")
		return false
	}

	shell := filepath.Base(shellPath)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		ui.ShowError("Failed to get home directory", err)
		return false
	}

	switch shell {
	case "zsh":
		configFile := filepath.Join(homeDir, ".zshrc")
		appendCompletionLine(configFile, "source <(codes completion zsh)")
	case "bash":
		configFile := filepath.Join(homeDir, ".bashrc")
		if runtime.GOOS == "darwin" {
			configFile = filepath.Join(homeDir, ".bash_profile")
		}
		appendCompletionLine(configFile, "source <(codes completion bash)")
	case "fish":
		completionDir := filepath.Join(homeDir, ".config", "fish", "completions")
		if err := os.MkdirAll(completionDir, 0755); err != nil {
			ui.ShowError("Failed to create fish completions directory", err)
			return false
		}
		completionFile := filepath.Join(completionDir, "codes.fish")
		if _, err := os.Stat(completionFile); err == nil {
			ui.ShowSuccess("Fish completion already installed at %s", completionFile)
			return true
		}
		content := "# codes CLI completion\ncodes completion fish | source\n"
		if err := os.WriteFile(completionFile, []byte(content), 0644); err != nil {
			ui.ShowError("Failed to write fish completion", err)
			return false
		}
		ui.ShowSuccess("Fish completion installed at %s", completionFile)
	default:
		ui.ShowWarning("Unsupported shell: %s, skipping completion setup", shell)
		ui.ShowInfo("  You can manually run: codes completion --help")
		return false
	}
	return true
}

// appendCompletionLine appends a completion source line to a shell config file if not already present.
func appendCompletionLine(configFile, completionLine string) {
	if data, err := os.ReadFile(configFile); err == nil {
		if strings.Contains(string(data), "codes completion") {
			ui.ShowSuccess("Shell completion already configured in %s", configFile)
			return
		}
	}

	f, err := os.OpenFile(configFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		ui.ShowError("Failed to write to "+configFile, err)
		return
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n# codes CLI completion\n%s\n", completionLine); err != nil {
		ui.ShowError("Failed to write completion config", err)
		return
	}

	ui.ShowSuccess("Shell completion installed in %s", configFile)
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

		// Check Claude version
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
		ui.ShowInfo("  ANTHROPIC_AUTH_TOKEN: %s...", authToken[:min(10, len(authToken))])
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
				// Prompt for configuration name
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Enter a name for this configuration (default: imported): ")
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				if input != "" {
					name = input
				}
			}

			// Create and test configuration
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

			// Save configuration
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

				// Show configurations with status
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

					// Check permissions setting
					permissionsText := "default"
					if config.ShouldSkipPermissions(&c) {
						permissionsText = "skip permissions"
					} else if c.SkipPermissions != nil && !*c.SkipPermissions {
						permissionsText = "use permissions"
					}

					// Get API endpoint
					apiURL := "unknown"
					if baseURL, exists := c.Env["ANTHROPIC_BASE_URL"]; exists {
						apiURL = baseURL
					}

					fmt.Printf("  %d. %s %s%s - %s [%s, %s]\n",
						i+1, statusIcon, c.Name, isDefault, apiURL, statusText, permissionsText)

					// Show environment variables (truncated for display)
					if len(c.Env) > 0 {
						fmt.Printf("      Environment Variables (%d):\n", len(c.Env))
						for envKey, envValue := range c.Env {
							// Truncate sensitive values
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

				// Test default configuration
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

func checkForUpdates() {
	// Apply any previously staged update (synchronous)
	if err := update.ApplyStaged(); err != nil {
		ui.ShowWarning("Failed to apply staged update: %v", err)
	}

	// Async version check
	mode := config.GetAutoUpdate()
	go update.AutoCheck(Version, mode)
}

// RunSelfUpdate performs a manual codes self-update.
func RunSelfUpdate() {
	ui.ShowHeader("codes Self-Update")
	if err := update.RunSelfUpdate(Version); err != nil {
		ui.ShowError(err.Error(), nil)
		os.Exit(1)
	}
}

// RunStart 快速启动 Claude Code
func RunStart(args []string) {
	var targetDir string

	if len(args) > 0 {
		input := args[0]

		// 检查是否是项目别名
		if project, exists := config.GetProject(input); exists {
			// Remote project → SSH
			if project.Remote != "" {
				host, ok := config.GetRemote(project.Remote)
				if !ok {
					ui.ShowError(fmt.Sprintf("Remote '%s' not found for project '%s'", project.Remote, input), nil)
					os.Exit(1)
				}
				ui.ShowInfo("Connecting to remote project: %s @ %s", input, host.UserAtHost())
				if err := remote.RunSSHInteractive(host, fmt.Sprintf("cd %s && codes", project.Path)); err != nil {
					ui.ShowError("SSH session failed", err)
					os.Exit(1)
				}
				return
			}
			targetDir = project.Path
			ui.ShowInfo("Using project: %s -> %s", input, targetDir)
		} else {
			// 作为路径处理
			absPath, err := filepath.Abs(input)
			if err != nil {
				ui.ShowError("Invalid path", err)
				os.Exit(1)
			}
			targetDir = absPath
		}

		// 验证目录是否存在
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			ui.ShowError("Directory does not exist", err)
			os.Exit(1)
		}
	} else {
		// 没有参数，根据配置决定使用哪个目录
		var err error
		behavior := config.GetDefaultBehavior()

		switch behavior {
		case "current":
			targetDir, err = os.Getwd()
			if err != nil {
				ui.ShowError("Failed to get current directory", err)
				os.Exit(1)
			}
			ui.ShowInfo("Using current directory: %s", targetDir)
		case "last":
			lastDir, err := config.GetLastWorkDir()
			if err != nil {
				ui.ShowError("Failed to get last working directory", err)
				os.Exit(1)
			}
			targetDir = lastDir
			ui.ShowInfo("Using last directory: %s", targetDir)
		case "home":
			homeDir, err := os.UserHomeDir()
			if err != nil {
				ui.ShowError("Failed to get home directory", err)
				os.Exit(1)
			}
			targetDir = homeDir
			ui.ShowInfo("Using home directory: %s", targetDir)
		default:
			// 默认使用当前目录
			targetDir, err = os.Getwd()
			if err != nil {
				ui.ShowError("Failed to get current directory", err)
				os.Exit(1)
			}
			ui.ShowInfo("Using current directory: %s", targetDir)
		}
	}

	// 保存当前目录为上次目录
	if err := config.SaveLastWorkDir(targetDir); err != nil {
		ui.ShowWarning("Failed to save working directory: %v", err)
	}

	// 启动 Claude
	runClaudeInDirectory(targetDir)
}

// RunProjectAdd 添加项目别名
func RunProjectAdd(name, path string, remoteName string) {
	entry := config.ProjectEntry{Path: path, Remote: remoteName}

	if remoteName != "" {
		// Remote project — verify remote exists, skip local path validation
		if _, ok := config.GetRemote(remoteName); !ok {
			ui.ShowError(fmt.Sprintf("Remote '%s' not found. Add it first with: codes remote add", remoteName), nil)
			return
		}

		if err := config.AddProjectEntry(name, entry); err != nil {
			ui.ShowError("Failed to add project", err)
			return
		}

		ui.ShowSuccess("Remote project '%s' added successfully!", name)
		ui.ShowInfo("Path: %s (on %s)", path, remoteName)
		ui.ShowInfo("Usage: codes start %s", name)
		return
	}

	// Local project — validate path
	absPath, err := filepath.Abs(path)
	if err != nil {
		ui.ShowError("Invalid path", err)
		return
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		ui.ShowError("Directory does not exist", err)
		return
	}

	entry.Path = absPath
	if err := config.AddProjectEntry(name, entry); err != nil {
		ui.ShowError("Failed to add project", err)
		return
	}

	ui.ShowSuccess("Project '%s' added successfully!", name)
	ui.ShowInfo("Path: %s", absPath)
	ui.ShowInfo("Usage: codes start %s", name)
}

// RunProjectRemove 删除项目别名
func RunProjectRemove(name string) {
	// 检查项目是否存在
	if _, exists := config.GetProjectPath(name); !exists {
		ui.ShowWarning("Project '%s' does not exist", name)
		return
	}

	// 删除项目
	if err := config.RemoveProject(name); err != nil {
		ui.ShowError("Failed to remove project", err)
		return
	}

	ui.ShowSuccess("Project '%s' removed successfully!", name)
}

// RunProjectList 列出所有项目
func RunProjectList() {
	projects, err := config.ListProjects()
	if err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Failed to load projects", err)
		return
	}

	if output.JSONMode {
		infos := make([]config.ProjectInfo, 0, len(projects))
		for name, entry := range projects {
			infos = append(infos, config.GetProjectInfoFromEntry(name, entry))
		}
		output.Print(infos, nil)
		return
	}

	if len(projects) == 0 {
		ui.ShowInfo("No projects configured yet")
		ui.ShowInfo("Add a project with: codes project add [name] [path]")
		return
	}

	fmt.Println()
	ui.ShowHeader("Configured Projects")
	fmt.Println()

	i := 1
	for name, entry := range projects {
		if entry.Remote != "" {
			ui.ShowInfo("%d. %s -> %s @ %s", i, name, entry.Path, entry.Remote)
		} else if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
			ui.ShowWarning("%d. %s -> %s (not found)", i, name, entry.Path)
		} else {
			ui.ShowInfo("%d. %s -> %s", i, name, entry.Path)
		}
		i++
	}

	fmt.Println()
	ui.ShowInfo("Start a project with: codes start <name>")
}

// RunProjectScan scans for existing Claude Code projects and imports them.
func RunProjectScan() {
	ui.ShowLoading("Scanning ~/.claude/projects/...")

	discovered, err := config.ScanClaudeProjects()
	if err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Failed to scan Claude projects", err)
		return
	}

	if len(discovered) == 0 {
		if output.JSONMode {
			output.Print(map[string]int{"added": 0, "skipped": 0}, nil)
			return
		}
		ui.ShowInfo("No Claude projects found in ~/.claude/projects/")
		return
	}

	added, skipped, err := config.ImportDiscoveredProjects(discovered)
	if err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Failed to import projects", err)
		return
	}

	if output.JSONMode {
		output.Print(map[string]int{"added": added, "skipped": skipped, "total": len(discovered)}, nil)
		return
	}

	fmt.Println()
	ui.ShowHeader("Claude Project Scan")
	fmt.Println()

	if added > 0 {
		ui.ShowSuccess("Imported %d new project(s)", added)
	}
	if skipped > 0 {
		ui.ShowInfo("Skipped %d (already in config)", skipped)
	}
	if added == 0 {
		ui.ShowInfo("All discovered projects are already configured")
	}
	fmt.Println()
}

// runClaudeInDirectory 在指定目录运行 Claude
func runClaudeInDirectory(dir string) {
	// 调用更新检查
	checkForUpdates()

	cmd := config.BuildClaudeCmd(dir)

	ui.ShowInfo("Working directory: %s", dir)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// RunTest 测试 API 配置
func RunTest(args []string) {
	ui.ShowHeader("API Configuration Test")
	fmt.Println()

	// 加载配置
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

	// 检查是否指定了特定配置
	if len(args) > 0 && args[0] != "" {
		// 测试特定配置
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
		// 测试所有配置
		ui.ShowInfo("Testing all %d configurations...", len(cfg.Profiles))
		testAllConfigurations(cfg.Profiles)
	}
}

// testSingleConfiguration 测试单个配置
func testSingleConfiguration(apiConfig *config.APIConfig) {
	fmt.Println()

	// 获取模型信息用于显示
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

	// 测试 API 连接
	ui.ShowLoading("Testing API connection...")
	if config.TestAPIConfig(*apiConfig) {
		ui.ShowSuccess("API connection successful!")
		apiConfig.Status = "active"
	} else {
		ui.ShowError("API connection failed", nil)
		apiConfig.Status = "inactive"
		ui.ShowWarning("Check your configuration and network connectivity")
	}

	// 保存更新后的状态
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config for update", err)
		return
	}

	// 更新配置状态
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

// testAllConfigurations 测试所有配置
func testAllConfigurations(configs []config.APIConfig) {
	results := make(map[string]bool)
	statuses := make(map[string]string)
	successCount := 0

	fmt.Println()
	for i := range configs {
		fmt.Printf("Testing %s...", configs[i].Name)

		// 获取模型信息
		envVars := config.GetEnvironmentVars(&configs[i])
		model := envVars["ANTHROPIC_MODEL"]
		if model == "" {
			model = envVars["ANTHROPIC_DEFAULT_HAIKU_MODEL"]
			if model == "" {
				model = "claude-3-haiku-20240307"
			}
		}

		// 测试 API 连接
		success := config.TestAPIConfig(configs[i])
		results[configs[i].Name] = success

		if success {
			fmt.Printf(" ✓ (Model: %s)\n", model)
			statuses[configs[i].Name] = "active"
			successCount++
		} else {
			fmt.Printf(" ✗ (Model: %s)\n", model)
			statuses[configs[i].Name] = "inactive"
		}
	}

	// 显示总结
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

	// 保存更新后的状态
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config for update", err)
		return
	}

	// 更新所有配置状态
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
}

// RunConfigSet 设置配置值
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

// RunConfigGet 获取配置值
func RunConfigGet(args []string) {
	if len(args) == 0 {
		// 显示所有设置
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

// RunDefaultBehaviorSet 设置默认行为
func RunDefaultBehaviorSet(behavior string) {
	// 验证值
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

	// 显示帮助信息
	fmt.Println()
	ui.ShowInfo("Examples:")
	ui.ShowInfo("  codes                    - Start Claude with %s directory", behavior)
	ui.ShowInfo("  codes start project-name - Start Claude in specific project")
	ui.ShowInfo("  codes start /path/to/dir - Start Claude in specific directory")
}

// RunDefaultBehaviorGet 获取当前默认行为
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

// RunDefaultBehaviorReset 重置默认行为
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

// RunSkipPermissionsSet 设置全局 skipPermissions
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

// RunSkipPermissionsGet 获取全局 skipPermissions 设置
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

// RunSkipPermissionsReset 重置全局 skipPermissions
func RunSkipPermissionsReset() {
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	oldValue := cfg.SkipPermissions
	cfg.SkipPermissions = false // 重置为 false

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

// RunProfileList 列出所有 profile 及其状态
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

// RunProfileRemove 删除指定 profile
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

	// If removed the default, switch to first available
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

// RunConfigReset 重置配置项
func RunConfigReset(args []string) {
	if len(args) == 0 {
		// 重置所有设置
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

// RunConfigList 列出配置键的可选值
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

// RunTerminalReset 重置终端设置为平台默认
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

// RunProjectAdd2 解析 0/1/2 参数后调用 RunProjectAdd
func RunProjectAdd2(args []string, remoteName string) {
	var name, path string

	switch len(args) {
	case 0:
		// 无参数：使用当前目录
		cwd, err := os.Getwd()
		if err != nil {
			ui.ShowError("Failed to get current directory", err)
			return
		}
		path = cwd
		name = filepath.Base(cwd)
	case 1:
		// 1 个参数：用作路径，从目录名推导名称
		absPath, err := filepath.Abs(args[0])
		if err != nil {
			ui.ShowError("Invalid path", err)
			return
		}
		path = absPath
		name = filepath.Base(absPath)
	case 2:
		// 2 个参数：原始行为
		name = args[0]
		path = args[1]
	}

	RunProjectAdd(name, path, remoteName)
}

// RunServe starts the MCP server mode.
func RunServe() {
	if err := mcpserver.RunServer(); err != nil {
		// EOF is expected when client disconnects
		if err.Error() != "server is closing: EOF" {
			ui.ShowError("MCP server error", err)
			os.Exit(1)
		}
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

// parseSSHAddress parses "user@host" or "host" into user and host parts.
func parseSSHAddress(address string) (user, host string) {
	if i := strings.Index(address, "@"); i >= 0 {
		return address[:i], address[i+1:]
	}
	return "", address
}

// RunRemoteAdd adds a new remote host.
func RunRemoteAdd(name, address string, port int, identity string) {
	user, host := parseSSHAddress(address)

	rh := config.RemoteHost{
		Name:     name,
		Host:     host,
		User:     user,
		Port:     port,
		Identity: identity,
	}

	if output.JSONMode {
		if err := config.AddRemote(rh); err != nil {
			output.PrintError(err)
			return
		}
		output.Print(map[string]interface{}{"added": true, "name": name}, nil)
		return
	}

	if err := config.AddRemote(rh); err != nil {
		ui.ShowError("Failed to add remote", err)
		return
	}

	ui.ShowSuccess("Remote '%s' added successfully!", name)
	ui.ShowInfo("Host: %s", rh.UserAtHost())
	if port != 0 {
		ui.ShowInfo("Port: %d", port)
	}
	if identity != "" {
		ui.ShowInfo("Identity: %s", identity)
	}
}

// RunRemoteRemove removes a remote host.
func RunRemoteRemove(name string) {
	if output.JSONMode {
		if err := config.RemoveRemote(name); err != nil {
			output.PrintError(err)
			return
		}
		output.Print(map[string]interface{}{"removed": true, "name": name}, nil)
		return
	}

	if err := config.RemoveRemote(name); err != nil {
		ui.ShowError("Failed to remove remote", err)
		return
	}
	ui.ShowSuccess("Remote '%s' removed", name)
}

// RunRemoteList lists all remote hosts.
func RunRemoteList() {
	remotes, err := config.ListRemotes()
	if err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Failed to list remotes", err)
		return
	}

	if output.JSONMode {
		output.Print(remotes, nil)
		return
	}

	if len(remotes) == 0 {
		ui.ShowInfo("No remote hosts configured")
		ui.ShowInfo("Add a remote with: codes remote add <name> <[user@]host>")
		return
	}

	fmt.Println()
	ui.ShowHeader("Remote Hosts")
	fmt.Println()

	for i, r := range remotes {
		info := r.UserAtHost()
		if r.Port != 0 {
			info += fmt.Sprintf(":%d", r.Port)
		}
		ui.ShowInfo("%d. %s → %s", i+1, r.Name, info)
	}

	fmt.Println()
	ui.ShowInfo("Check status with: codes remote status <name>")
}

// RunRemoteStatus shows the status of a remote host.
func RunRemoteStatus(name string) {
	host, ok := config.GetRemote(name)
	if !ok {
		if output.JSONMode {
			output.PrintError(fmt.Errorf("remote %q not found", name))
			return
		}
		ui.ShowError(fmt.Sprintf("Remote '%s' not found", name), nil)
		return
	}

	if !output.JSONMode {
		ui.ShowInfo("Checking %s (%s)...", name, host.UserAtHost())
	}

	// Test connection
	if err := remote.TestConnection(host); err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Connection failed", err)
		return
	}

	status, err := remote.CheckRemoteStatus(host)
	if err != nil {
		if output.JSONMode {
			output.PrintError(err)
			return
		}
		ui.ShowError("Failed to check status", err)
		return
	}

	if output.JSONMode {
		output.Print(status, nil)
		return
	}

	fmt.Println()
	ui.ShowSuccess("Connection: OK")
	ui.ShowInfo("OS: %s", status.OS)
	ui.ShowInfo("Arch: %s", status.Arch)

	if status.CodesInstalled {
		ui.ShowSuccess("codes: installed (%s)", status.CodesVersion)
	} else {
		ui.ShowWarning("codes: not installed")
	}

	if status.ClaudeInstalled {
		ui.ShowSuccess("claude: installed")
	} else {
		ui.ShowWarning("claude: not installed")
	}
}

// RunRemoteInstall installs codes on a remote host.
func RunRemoteInstall(name string) {
	host, ok := config.GetRemote(name)
	if !ok {
		ui.ShowError(fmt.Sprintf("Remote '%s' not found", name), nil)
		return
	}

	ui.ShowLoading("Installing codes on %s...", host.UserAtHost())

	out, err := remote.InstallOnRemote(host)
	if err != nil {
		ui.ShowError("Installation failed", err)
		return
	}
	if out != "" {
		fmt.Println(out)
	}

	ui.ShowSuccess("codes installed on %s!", host.UserAtHost())
}

// RunRemoteSync syncs profiles to a remote host.
func RunRemoteSync(name string) {
	host, ok := config.GetRemote(name)
	if !ok {
		ui.ShowError(fmt.Sprintf("Remote '%s' not found", name), nil)
		return
	}

	ui.ShowLoading("Syncing profiles to %s...", host.UserAtHost())

	if err := remote.SyncProfiles(host); err != nil {
		ui.ShowError("Sync failed", err)
		return
	}

	ui.ShowSuccess("Profiles synced to %s!", host.UserAtHost())
}

// RunRemoteSetup runs install + sync on a remote host.
func RunRemoteSetup(name string) {
	host, ok := config.GetRemote(name)
	if !ok {
		ui.ShowError(fmt.Sprintf("Remote '%s' not found", name), nil)
		return
	}

	// Step 1: Install codes
	ui.ShowLoading("Installing codes on %s...", host.UserAtHost())
	if _, err := remote.InstallOnRemote(host); err != nil {
		ui.ShowError("Installation failed", err)
		return
	}
	ui.ShowSuccess("codes installed!")

	// Step 2: Install Claude CLI
	ui.ShowLoading("Installing Claude CLI...")
	out, err := remote.InstallClaudeOnRemote(host)
	if err != nil {
		ui.ShowWarning("Claude CLI: %v", err)
	} else {
		if strings.Contains(out, "already installed") {
			ui.ShowSuccess("Claude CLI already installed")
		} else {
			ui.ShowSuccess("Claude CLI installed!")
		}
	}

	// Step 3: Sync profiles
	ui.ShowLoading("Syncing profiles...")
	if err := remote.SyncProfiles(host); err != nil {
		ui.ShowError("Sync failed", err)
		return
	}
	ui.ShowSuccess("Profiles synced!")

	fmt.Println()
	ui.ShowSuccess("Remote '%s' is ready!", name)
	ui.ShowInfo("Connect with: codes remote ssh %s", name)
}

// RunRemoteSSH opens an interactive SSH session on the remote host.
func RunRemoteSSH(name string, project string) {
	host, ok := config.GetRemote(name)
	if !ok {
		ui.ShowError(fmt.Sprintf("Remote '%s' not found", name), nil)
		return
	}

	// Build remote command
	var cmd string
	if project != "" {
		cmd = fmt.Sprintf("cd %s && codes", project)
	} else {
		cmd = "codes"
	}

	if err := remote.RunSSHInteractive(host, cmd); err != nil {
		ui.ShowError("SSH session failed", err)
	}
}
