package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"codes/internal/config"
	"codes/internal/ui"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func RunVersion() {
	fmt.Printf("codes version dev (built unknown)\n")
}

func RunSelect() {
	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	fmt.Println()
	ui.ShowHeader("Available Claude Configurations")
	fmt.Println()

	for i, c := range cfg.Configs {
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

	if selectedIdx, err := strconv.Atoi(selection); err == nil && selectedIdx >= 1 && selectedIdx <= len(cfg.Configs) {
		selectedConfig := cfg.Configs[selectedIdx-1]
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

func RunUpdate() {
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
		configData.Configs = []config.APIConfig{}
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
	for _, c := range configData.Configs {
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
	configData.Configs = append(configData.Configs, newConfig)

	// 如果这是第一个配置，设置为默认
	if len(configData.Configs) == 1 {
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

func RunInstall() {
	ui.ShowHeader("Installing codes CLI")

	// 获取当前可执行文件路径
	executablePath, err := os.Executable()
	if err != nil {
		ui.ShowError("Failed to get executable path", err)
		return
	}

	// 确定安装目标路径
	var targetDir string
	var installPath string

	switch runtime.GOOS {
	case "windows":
		// Windows: 安装到用户目录下的Scripts目录
		homeDir, _ := os.UserHomeDir()
		targetDir = filepath.Join(homeDir, "go", "bin")
		installPath = filepath.Join(targetDir, "codes.exe")
	default:
		// Linux/macOS: 安装到/usr/local/bin或~/bin
		if ui.CanWriteTo("/usr/local/bin") {
			targetDir = "/usr/local/bin"
			installPath = filepath.Join(targetDir, "codes")
		} else {
			homeDir, _ := os.UserHomeDir()
			targetDir = filepath.Join(homeDir, "bin")
			installPath = filepath.Join(targetDir, "codes")
		}
	}

	ui.ShowInfo("Installing to: %s", installPath)

	// 创建目标目录
	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		ui.ShowError("Failed to create target directory", err)
		return
	}

	// 复制文件
	sourceData, err := os.ReadFile(executablePath)
	if err != nil {
		ui.ShowError("Failed to read executable", err)
		return
	}

	err = os.WriteFile(installPath, sourceData, 0755)
	if err != nil {
		ui.ShowError("Failed to write to target location", err)
		return
	}

	// 验证安装
	if _, err := os.Stat(installPath); err == nil {
		ui.ShowSuccess("codes installed successfully!")
		ui.ShowInfo("Installed to: %s", installPath)

		// 提示添加到PATH
		switch runtime.GOOS {
		case "windows":
			ui.ShowInfo("Add %s to your PATH environment variable", targetDir)
		default:
			if targetDir != "/usr/local/bin" {
				ui.ShowInfo("Add %s to your PATH in your shell profile", targetDir)
			}
		}
	} else {
		ui.ShowError("Installation verification failed", err)
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
	for _, c := range cfg.Configs {
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

func RunInit() {
	ui.ShowHeader("Codes CLI Environment Check")
	fmt.Println()

	allGood := true

	// 1. Check if Claude CLI is installed
	ui.ShowInfo("Checking Claude CLI installation...")
	if _, err := exec.LookPath("claude"); err != nil {
		ui.ShowError("✗ Claude CLI not found", nil)
		ui.ShowWarning("  Run 'codes update' to install Claude CLI")
		allGood = false
	} else {
		ui.ShowSuccess("✓ Claude CLI is installed")

		// Check Claude version
		cmd := exec.Command("claude", "--version")
		output, err := cmd.Output()
		if err == nil {
			version := strings.TrimSpace(string(output))
			ui.ShowInfo("  Version: %s", version)
		}
	}
	fmt.Println()

	// 2. Check for existing environment variables
	ui.ShowInfo("Checking for existing Claude configuration...")
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	authToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")

	hasEnvConfig := false
	if baseURL != "" && authToken != "" {
		ui.ShowSuccess("✓ Found existing configuration in environment variables")
		ui.ShowInfo("  ANTHROPIC_BASE_URL: %s", baseURL)
		ui.ShowInfo("  ANTHROPIC_AUTH_TOKEN: %s...", authToken[:min(10, len(authToken))])
		hasEnvConfig = true
	} else if baseURL != "" || authToken != "" {
		ui.ShowWarning("⚠ Incomplete environment configuration detected")
		if baseURL != "" {
			ui.ShowInfo("  ANTHROPIC_BASE_URL: %s", baseURL)
		}
		if authToken != "" {
			ui.ShowInfo("  ANTHROPIC_AUTH_TOKEN: configured")
		}
	}
	fmt.Println()

	// 3. Check if config file exists
	ui.ShowInfo("Checking configuration file...")
	configExists := false
	if _, err := os.Stat(config.ConfigPath); err == nil {
		configExists = true
	}

	// If env config exists but no codes config, offer to import
	if hasEnvConfig && !configExists {
		fmt.Println()
		ui.ShowInfo("Would you like to import this configuration? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "y" || response == "yes" {
			// Prompt for configuration name
			fmt.Print("Enter a name for this configuration (default: imported): ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				name = "imported"
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
			cfg.Configs = []config.APIConfig{testConfig}
			cfg.Default = name

			if config.TestAPIConfig(testConfig) {
				ui.ShowSuccess("✓ API connection successful!")
				testConfig.Status = "active"
			} else {
				ui.ShowWarning("⚠ API connection failed, but configuration will be saved")
				testConfig.Status = "inactive"
			}

			cfg.Configs[0] = testConfig

			// Save configuration
			if err := config.SaveConfig(&cfg); err != nil {
				ui.ShowError("Failed to save configuration", err)
				allGood = false
			} else {
				ui.ShowSuccess("✓ Configuration imported successfully!")
				ui.ShowInfo("  Name: %s", name)
				ui.ShowInfo("  Status: %s", testConfig.Status)
				configExists = true
			}
			fmt.Println()
		}
	}

	// Continue with normal config check
	if !configExists {
		ui.ShowError("✗ Configuration file not found", nil)
		ui.ShowInfo("  Expected location: %s", config.ConfigPath)
		ui.ShowWarning("  Run 'codes add' to create your first configuration")
		allGood = false
	} else {
		ui.ShowSuccess("✓ Configuration file exists")
		ui.ShowInfo("  Location: %s", config.ConfigPath)

		// 4. Validate configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			ui.ShowError("✗ Failed to load configuration", err)
			ui.ShowWarning("  Your config file may be corrupted")
			allGood = false
		} else {
			if len(cfg.Configs) == 0 {
				ui.ShowWarning("✗ No configurations found in config file")
				ui.ShowWarning("  Run 'codes add' to add a configuration")
				allGood = false
			} else {
				ui.ShowSuccess("✓ Found %d configuration(s)", len(cfg.Configs))

				// Show configurations with status
				fmt.Println()
				ui.ShowInfo("Configurations:")
				for i, c := range cfg.Configs {
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

				// 4. Test default configuration
				if cfg.Default != "" {
					fmt.Println()
					ui.ShowInfo("Testing default configuration '%s'...", cfg.Default)

					var defaultConfig *config.APIConfig
					for i := range cfg.Configs {
						if cfg.Configs[i].Name == cfg.Default {
							defaultConfig = &cfg.Configs[i]
							break
						}
					}

					if defaultConfig != nil {
						if config.TestAPIConfig(*defaultConfig) {
							ui.ShowSuccess("✓ Default configuration is working")
						} else {
							ui.ShowWarning("✗ Default configuration test failed")
							ui.ShowWarning("  API may be unreachable or credentials may be invalid")
							ui.ShowInfo("  Run 'codes add' to add a new configuration")
							allGood = false
						}
					} else {
						ui.ShowWarning("✗ Default configuration '%s' not found", cfg.Default)
						ui.ShowWarning("  Run 'codes select' to choose a valid configuration")
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
		ui.ShowSuccess("✓ All checks passed! You're ready to use codes.")
		fmt.Println()
		ui.ShowInfo("Quick commands:")
		ui.ShowInfo("  codes          - Run Claude with current configuration")
		ui.ShowInfo("  codes select   - Switch between configurations")
		ui.ShowInfo("  codes add      - Add a new configuration")
	} else {
		ui.ShowWarning("⚠ Some checks failed. Please review the messages above.")
		fmt.Println()
		ui.ShowInfo("Suggested actions:")
		if _, err := exec.LookPath("claude"); err != nil {
			ui.ShowInfo("  1. Install Claude CLI: codes update")
		}
		if _, err := os.Stat(config.ConfigPath); err != nil {
			ui.ShowInfo("  2. Add a configuration: codes add")
		}
	}
}

func checkForUpdates() {
	// 检查codes CLI更新
	go func() {
		// 简单的版本检查逻辑
		// 这里可以集成GitHub API检查最新版本
		// 目前只是占位符
		// 可以通过检查GitHub releases API来获取最新版本
		// 例如: https://api.github.com/repos/{owner}/{repo}/releases/latest
		// 然后与当前版本比较，提示用户更新
		//
		// 示例实现:
		// resp, err := http.Get("https://api.github.com/repos/yourusername/codes/releases/latest")
		// if err != nil {
		//     return
		// }
		// defer resp.Body.Close()
		//
		// var release struct {
		//     TagName string `json:"tag_name"`
		// }
		// if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		//     return
		// }
		//
		// if release.TagName != "dev" && release.TagName != currentVersion {
		//     ui.ShowInfo("New version %s available! Run 'codes update' to upgrade.", release.TagName)
		// }
	}()
}

// RunStart 快速启动 Claude Code
func RunStart(args []string) {
	var targetDir string

	if len(args) > 0 {
		input := args[0]

		// 检查是否是项目别名
		if projectPath, exists := config.GetProjectPath(input); exists {
			targetDir = projectPath
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
func RunProjectAdd(name, path string) {
	// 转换为绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		ui.ShowError("Invalid path", err)
		return
	}

	// 验证目录是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		ui.ShowError("Directory does not exist", err)
		return
	}

	// 添加项目
	if err := config.AddProject(name, absPath); err != nil {
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
		ui.ShowError("Failed to load projects", err)
		return
	}

	if len(projects) == 0 {
		ui.ShowInfo("No projects configured yet")
		ui.ShowInfo("Add a project with: codes project add <name> <path>")
		return
	}

	fmt.Println()
	ui.ShowHeader("Configured Projects")
	fmt.Println()

	i := 1
	for name, path := range projects {
		// 检查目录是否仍然存在
		if _, err := os.Stat(path); os.IsNotExist(err) {
			ui.ShowWarning("%d. %s -> %s (not found)", i, name, path)
		} else {
			ui.ShowInfo("%d. %s -> %s", i, name, path)
		}
		i++
	}

	fmt.Println()
	ui.ShowInfo("Start a project with: codes start <name>")
}

// runClaudeInDirectory 在指定目录运行 Claude
func runClaudeInDirectory(dir string) {
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
	for _, c := range cfg.Configs {
		if c.Name == cfg.Default {
			selectedConfig = c
			break
		}
	}

	// Set environment variables
	config.SetEnvironmentVarsWithConfig(&selectedConfig, cfg)

	ui.ShowInfo("Using configuration: %s", selectedConfig.Name)
	ui.ShowInfo("Working directory: %s", dir)

	// Build claude command with or without --dangerously-skip-permissions
	var claudeArgs []string
	if config.ShouldSkipPermissionsWithConfig(&selectedConfig, cfg) {
		claudeArgs = []string{"--dangerously-skip-permissions"}
	}

	cmd := exec.Command("claude", claudeArgs...)
	cmd.Dir = dir // 设置工作目录，而不是作为参数传递
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

	if len(cfg.Configs) == 0 {
		ui.ShowError("No configurations found", nil)
		ui.ShowInfo("Run 'codes add' to add a configuration first")
		return
	}

	// 检查是否指定了特定配置
	if len(args) > 0 && args[0] != "" {
		// 测试特定配置
		configName := args[0]
		var targetConfig *config.APIConfig
		for i := range cfg.Configs {
			if cfg.Configs[i].Name == configName {
				targetConfig = &cfg.Configs[i]
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
		ui.ShowInfo("Testing all %d configurations...", len(cfg.Configs))
		testAllConfigurations(cfg.Configs)
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
		ui.ShowSuccess("✓ API connection successful!")
		apiConfig.Status = "active"
	} else {
		ui.ShowError("✗ API connection failed", nil)
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
	for i := range cfg.Configs {
		if cfg.Configs[i].Name == apiConfig.Name {
			cfg.Configs[i].Status = apiConfig.Status
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
		ui.ShowInfo("Use 'codes test <config-name>' to test individual configurations")
	}

	// 保存更新后的状态
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config for update", err)
		return
	}

	// 更新所有配置状态
	updated := false
	for i := range cfg.Configs {
		if newStatus, ok := statuses[cfg.Configs[i].Name]; ok {
			if cfg.Configs[i].Status != newStatus {
				cfg.Configs[i].Status = newStatus
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
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	switch key {
	case "defaultBehavior":
		// 验证值
		if value != "current" && value != "last" && value != "home" {
			ui.ShowError("Invalid value for defaultBehavior. Must be 'current', 'last', or 'home'", nil)
			return
		}
		cfg.DefaultBehavior = value
		ui.ShowSuccess("Default behavior set to: %s", value)
	default:
		ui.ShowError("Unknown configuration key: %s", nil)
		fmt.Printf("Available keys: defaultBehavior\n")
		return
	}

	if err := config.SaveConfig(cfg); err != nil {
		ui.ShowError("Error saving config", err)
		return
	}
}

// RunConfigGet 获取配置值
func RunConfigGet(args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Error loading config", err)
		return
	}

	if len(args) > 0 {
		// 显示特定配置
		key := args[0]
		switch key {
		case "defaultBehavior":
			fmt.Printf("%s: %s\n", key, cfg.DefaultBehavior)
		default:
			ui.ShowError("Unknown configuration key: %s", nil)
			fmt.Printf("Available keys: defaultBehavior\n")
			return
		}
	} else {
		// 显示所有配置
		fmt.Println("Current configuration:")
		fmt.Printf("  defaultBehavior: %s\n", cfg.DefaultBehavior)
		fmt.Printf("  skipPermissions: %v\n", cfg.SkipPermissions)
		fmt.Printf("  lastWorkDir: %s\n", cfg.LastWorkDir)
		fmt.Printf("  default: %s\n", cfg.Default)
		fmt.Printf("  projects: %d configured\n", len(cfg.Projects))
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
	ui.ShowInfo("  codes defaultbehavior set <current|last|home>")
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
