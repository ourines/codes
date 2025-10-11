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
		if c.Name == cfg.Default {
			if c.Status == "active" {
				ui.ShowCurrentConfig(i+1, c.Name, c.AnthropicBaseURL)
				ui.ShowInfo("     Status: Active")
			} else if c.Status == "inactive" {
				ui.ShowCurrentConfig(i+1, c.Name, c.AnthropicBaseURL)
				ui.ShowWarning("     Status: Inactive")
			} else {
				ui.ShowCurrentConfig(i+1, c.Name, c.AnthropicBaseURL)
			}
		} else {
			if c.Status == "active" {
				ui.ShowConfigOption(i+1, c.Name, c.AnthropicBaseURL)
				ui.ShowInfo("     Status: Active")
			} else if c.Status == "inactive" {
				ui.ShowConfigOption(i+1, c.Name, c.AnthropicBaseURL)
				ui.ShowWarning("     Status: Inactive")
			} else {
				ui.ShowConfigOption(i+1, c.Name, c.AnthropicBaseURL)
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
		ui.ShowInfo("API: %s", selectedConfig.AnthropicBaseURL)

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

	// 显示最新5个版本
	ui.ShowInfo("Latest versions:")
	endIndex := 5
	if len(versions) < 5 {
		endIndex = len(versions)
	}

	for i := 0; i < endIndex; i++ {
		ui.ShowVersionItem(i+1, versions[i])
	}

	fmt.Println()
	fmt.Printf("Select version to install (1-%d): ", endIndex)
	reader := bufio.NewReader(os.Stdin)
	selection, _ := reader.ReadString('\n')
	selection = strings.TrimSpace(selection)

	if selectedIdx, err := strconv.Atoi(selection); err == nil && selectedIdx >= 1 && selectedIdx <= endIndex {
		selectedVersion := versions[selectedIdx-1]
		ui.ShowLoading("Installing Claude %s...", selectedVersion)
		installClaude(selectedVersion)
	} else {
		ui.ShowWarning("Invalid selection. Installing latest...")
		installClaude("latest")
	}
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

	// 获取API URL
	fmt.Print("Enter ANTHROPIC_BASE_URL: ")
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		ui.ShowError("Base URL cannot be empty", nil)
		return
	}

	// 验证URL格式
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		ui.ShowError("Invalid URL format. Must start with http:// or https://", nil)
		return
	}

	// 获取API Token
	fmt.Print("Enter ANTHROPIC_AUTH_TOKEN: ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)
	if token == "" {
		ui.ShowError("Auth token cannot be empty", nil)
		return
	}

	// 测试API连接
	ui.ShowLoading("Testing API connection")
	testConfig := config.APIConfig{
		Name:               name,
		AnthropicBaseURL:   baseURL,
		AnthropicAuthToken: token,
	}

	if config.TestAPIConfig(testConfig) {
		ui.ShowSuccess("API connection successful!")
		testConfig.Status = "active"
	} else {
		ui.ShowWarning("API connection failed. Configuration added but marked as inactive")
		testConfig.Status = "inactive"
	}

	// 添加新配置
	configData.Configs = append(configData.Configs, testConfig)

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
	if testConfig.Status == "active" {
		ui.ShowInfo("Status: Active")
	} else {
		ui.ShowWarning("Status: Inactive (API test failed)")
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
		ui.ShowError("Error loading config: %v", err)
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
	os.Setenv("ANTHROPIC_BASE_URL", selectedConfig.AnthropicBaseURL)
	os.Setenv("ANTHROPIC_AUTH_TOKEN", selectedConfig.AnthropicAuthToken)

	ui.ShowInfo("Using configuration: %s (%s)", selectedConfig.Name, selectedConfig.AnthropicBaseURL)
	// Run claude with dangerous permissions
	cmd := exec.Command("claude", append([]string{"--dangerously-skip-permissions"}, args...)...)
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

	// 2. Check if config file exists
	ui.ShowInfo("Checking configuration file...")
	if _, err := os.Stat(config.ConfigPath); err != nil {
		ui.ShowError("✗ Configuration file not found", nil)
		ui.ShowInfo("  Expected location: %s", config.ConfigPath)
		ui.ShowWarning("  Run 'codes add' to create your first configuration")
		allGood = false
	} else {
		ui.ShowSuccess("✓ Configuration file exists")
		ui.ShowInfo("  Location: %s", config.ConfigPath)

		// 3. Validate configuration
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

					fmt.Printf("  %d. %s %s%s - %s [%s]\n",
						i+1, statusIcon, c.Name, isDefault, c.AnthropicBaseURL, statusText)
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
