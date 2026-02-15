package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Profiles        []APIConfig       `json:"profiles"`
	Default         string            `json:"default"`
	SkipPermissions bool              `json:"skipPermissions,omitempty"` // 全局是否跳过权限检查
	Projects        map[string]ProjectEntry `json:"projects,omitempty"`   // 项目别名 -> 项目条目
	LastWorkDir     string            `json:"lastWorkDir,omitempty"`     // 上次工作目录
	DefaultBehavior string            `json:"defaultBehavior,omitempty"` // 默认启动行为: "current", "last", "home"
	Terminal        string            `json:"terminal,omitempty"`        // 终端模拟器: "terminal", "iterm", "warp", ��自定义命令
	Remotes         []RemoteHost      `json:"remotes,omitempty"`         // 远程 SSH 主机
	ProjectsDir     string            `json:"projects_dir,omitempty"`    // git clone 默认目标目录
	AutoUpdate      string            `json:"auto_update,omitempty"`     // 自动更新模式: "notify", "silent", "off"
	Editor          string            `json:"editor,omitempty"`          // 编辑器命令: "code", "cursor", "zed", etc.
	Webhooks        []WebhookConfig   `json:"webhooks,omitempty"`        // Webhook 通知配置
}

// WebhookConfig represents a webhook notification endpoint.
type WebhookConfig struct {
	Name   string   `json:"name"`             // 配置名称（可选，用于管理多个webhook）
	URL    string   `json:"url"`              // Webhook URL
	Format string   `json:"format,omitempty"` // "slack" or "feishu" (默认 "slack")
	Events []string `json:"events,omitempty"` // 事件过滤 ["task_completed", "task_failed"] (空表示全部)
}

// RemoteHost represents a remote SSH host configuration.
type RemoteHost struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	User     string `json:"user,omitempty"`
	Port     int    `json:"port,omitempty"`
	Identity string `json:"identity,omitempty"`
}

// UserAtHost returns the SSH connection string (e.g., "user@host" or just "host").
func (r RemoteHost) UserAtHost() string {
	if r.User != "" {
		return r.User + "@" + r.Host
	}
	return r.Host
}

// ProjectEntry represents a project with an optional remote host.
type ProjectEntry struct {
	Path   string        `json:"path"`
	Remote string        `json:"remote,omitempty"` // remote host name, empty = local
	Links  []ProjectLink `json:"links,omitempty"`  // linked projects
}

// UnmarshalJSON supports both old string format and new object format.
//
//	Old: "projects": {"myapp": "/path/to/app"}
//	New: "projects": {"myapp": {"path": "/path", "remote": "hk"}}
func (p *ProjectEntry) UnmarshalJSON(data []byte) error {
	// Try string first (old format)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		p.Path = s
		p.Remote = ""
		return nil
	}
	// Try object (new format)
	type Alias ProjectEntry
	return json.Unmarshal(data, (*Alias)(p))
}

// MarshalJSON saves local projects as plain string (backward compat),
// remote or linked projects as object.
func (p ProjectEntry) MarshalJSON() ([]byte, error) {
	if p.Remote == "" && len(p.Links) == 0 {
		return json.Marshal(p.Path)
	}
	type Alias ProjectEntry
	return json.Marshal(Alias(p))
}

type APIConfig struct {
	Name            string            `json:"name"`
	Env             map[string]string `json:"env,omitempty"`             // 环境变量映射
	SkipPermissions *bool             `json:"skipPermissions,omitempty"` // 单独配置是否跳过权限检查，nil 表示使用全局设置
	Status          string            `json:"status,omitempty"`          // "active", "inactive", "unknown"
}

// UnmarshalJSON implements custom JSON unmarshaling for APIConfig to support
// backward compatibility with the old flat config format where ANTHROPIC_BASE_URL
// and ANTHROPIC_AUTH_TOKEN were top-level fields instead of nested under "env".
func (a *APIConfig) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion
	type Alias APIConfig
	aux := &struct {
		*Alias
		// Legacy flat fields from old config format
		AnthropicBaseURL   string `json:"ANTHROPIC_BASE_URL"`
		AnthropicAuthToken string `json:"ANTHROPIC_AUTH_TOKEN"`
	}{
		Alias: (*Alias)(a),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Migrate legacy flat fields into the env map
	if a.Env == nil {
		a.Env = make(map[string]string)
	}
	if aux.AnthropicBaseURL != "" {
		if _, exists := a.Env["ANTHROPIC_BASE_URL"]; !exists {
			a.Env["ANTHROPIC_BASE_URL"] = aux.AnthropicBaseURL
		}
	}
	if aux.AnthropicAuthToken != "" {
		if _, exists := a.Env["ANTHROPIC_AUTH_TOKEN"]; !exists {
			a.Env["ANTHROPIC_AUTH_TOKEN"] = aux.AnthropicAuthToken
		}
	}

	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling for Config to support
// backward compatibility with the old "configs" field name.
func (c *Config) UnmarshalJSON(data []byte) error {
	type Alias Config
	aux := &struct {
		*Alias
		// Legacy field name
		Configs []APIConfig `json:"configs"`
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Migrate legacy "configs" field into "profiles"
	if len(c.Profiles) == 0 && len(aux.Configs) > 0 {
		c.Profiles = aux.Configs
	}

	return nil
}

var ConfigPath string

func init() {
	// 首先检查项目根目录的配置文件
	pwd, _ := os.Getwd()
	projectConfig := filepath.Join(pwd, "config.json")
	if _, err := os.Stat(projectConfig); err == nil {
		ConfigPath = projectConfig
	} else {
		// 回退到用户目录
		homeDir, _ := os.UserHomeDir()
		ConfigPath = filepath.Join(homeDir, ".codes", "config.json")
	}
}

func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func SaveConfig(config *Config) error {
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(ConfigPath)
	os.MkdirAll(dir, 0755)
	return os.WriteFile(ConfigPath, data, 0644)
}

func TestAPIConfig(config APIConfig) bool {
	// 使用Claude API的实际端点进行测试
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 获取环境变量
	envVars := GetEnvironmentVars(&config)

	// Claude API的messages端点
	apiURL := envVars["ANTHROPIC_BASE_URL"]
	if apiURL == "" {
		apiURL = "https://api.anthropic.com" // 默认值
	}
	if apiURL[len(apiURL)-1] != '/' {
		apiURL += "/"
	}
	apiURL += "v1/messages"

	// 获取模型名称
	model := envVars["ANTHROPIC_MODEL"]
	if model == "" {
		// 尝试获取默认模型
		model = envVars["ANTHROPIC_DEFAULT_HAIKU_MODEL"]
		if model == "" {
			// 最后回退到默认模型
			model = "claude-3-haiku-20240307"
		}
	}

	// 构建测试请求体
	type testMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type testRequest struct {
		Model     string        `json:"model"`
		MaxTokens int           `json:"max_tokens"`
		Messages  []testMessage `json:"messages"`
	}

	reqBody := testRequest{
		Model:     model,
		MaxTokens: 10,
		Messages:  []testMessage{{Role: "user", Content: "Hello"}},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return false
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return false
	}

	// 设置Claude API头
	authToken := envVars["ANTHROPIC_AUTH_TOKEN"]
	if authToken == "" {
		authToken = envVars["ANTHROPIC_API_KEY"] // 备选方案
	}

	if authToken != "" {
		req.Header.Set("x-api-key", authToken)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-dangerous-disable-guardrails", "true") // 测试时禁用护栏

	resp, err := client.Do(req)
	if err != nil {
		// 如果POST失败，尝试HEAD请求测试连接
		return testBasicConnectivity(config)
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode == http.StatusOK {
		// 尝试读取响应体确认是有效的Claude响应
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			// 检查是否有content字段，这是Claude响应的特征
			if _, hasContent := result["content"]; hasContent {
				return true
			}
		}
		return true // 即使解析失败，状态码正确也认为API可用
	}

	// 401表示API可达但认证失败，也算有效
	// 400表示API可达但请求有问题
	return resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == 400
}

func testBasicConnectivity(config APIConfig) bool {
	// 简单的连接性测试
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	// 获取环境变量
	envVars := GetEnvironmentVars(&config)

	// 尝试连接基础URL
	testURL := envVars["ANTHROPIC_BASE_URL"]
	if testURL == "" {
		testURL = "https://api.anthropic.com" // 默认值
	}
	if testURL[len(testURL)-1] != '/' {
		testURL += "/"
	}

	req, err := http.NewRequest("HEAD", testURL, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500 // 任何非服务器错误都算作可达
}

// SaveLastWorkDir 保存上次工作目录
func SaveLastWorkDir(dir string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	cfg.LastWorkDir = dir
	return SaveConfig(cfg)
}

// GetLastWorkDir 获取上次工作目录
func GetLastWorkDir() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}

	if cfg.LastWorkDir == "" {
		homeDir, _ := os.UserHomeDir()
		return homeDir, nil
	}

	return cfg.LastWorkDir, nil
}

// AddProject 添加项目别名
func AddProject(name, path string) error {
	return AddProjectEntry(name, ProjectEntry{Path: path})
}

// AddProjectEntry 添加项目条目（支持 remote）
func AddProjectEntry(name string, entry ProjectEntry) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	if cfg.Projects == nil {
		cfg.Projects = make(map[string]ProjectEntry)
	}

	cfg.Projects[name] = entry
	return SaveConfig(cfg)
}

// RemoveProject 删除项目别名
func RemoveProject(name string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	if cfg.Projects == nil {
		return nil
	}

	delete(cfg.Projects, name)
	return SaveConfig(cfg)
}

// GetProjectPath 获取项目路径
func GetProjectPath(name string) (string, bool) {
	entry, exists := GetProject(name)
	if !exists {
		return "", false
	}
	return entry.Path, true
}

// GetProject 获取完整项目条目
func GetProject(name string) (ProjectEntry, bool) {
	cfg, err := LoadConfig()
	if err != nil {
		return ProjectEntry{}, false
	}

	if cfg.Projects == nil {
		return ProjectEntry{}, false
	}

	entry, exists := cfg.Projects[name]
	return entry, exists
}

// ListProjects 列出所有项目（返回 name → ProjectEntry）
func ListProjects() (map[string]ProjectEntry, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	if cfg.Projects == nil {
		return make(map[string]ProjectEntry), nil
	}

	return cfg.Projects, nil
}

// ShouldSkipPermissions 判断是否应该跳过权限检查
func ShouldSkipPermissions(apiConfig *APIConfig) bool {
	return ShouldSkipPermissionsWithConfig(apiConfig, nil)
}

// ShouldSkipPermissionsWithConfig 使用已加载的配置判断是否应该跳过权限检查
func ShouldSkipPermissionsWithConfig(apiConfig *APIConfig, cfg *Config) bool {
	// 如果没有提供配置，加载配置
	var loadedConfig *Config
	if cfg == nil {
		var err error
		loadedConfig, err = LoadConfig()
		if err != nil {
			return false
		}
		cfg = loadedConfig
	}

	// 如果 API 配置中有单独的设置，使用单独设置
	if apiConfig.SkipPermissions != nil {
		return *apiConfig.SkipPermissions
	}

	// 否则使用全局设置
	return cfg.SkipPermissions
}

// GetEnvironmentVars 获取配置的所有环境变量
func GetEnvironmentVars(apiConfig *APIConfig) map[string]string {
	envVars := make(map[string]string)

	// 添加所有配置的环境变量
	for key, value := range apiConfig.Env {
		envVars[key] = value
	}

	return envVars
}

// SetEnvironmentVars 设置环境变量到当前进程
func SetEnvironmentVars(apiConfig *APIConfig) {
	SetEnvironmentVarsWithConfig(apiConfig)
}

// SetEnvironmentVarsWithConfig 使用已加载的配置设置环境变量
func SetEnvironmentVarsWithConfig(apiConfig *APIConfig) {
	envVars := GetEnvironmentVars(apiConfig)
	for key, value := range envVars {
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to set environment variable %s: %v\n", key, err)
		}
	}
}

// BuildClaudeCmd creates an *exec.Cmd for launching Claude Code in the given directory.
// It loads the current config, sets environment variables, and applies skip-permissions if configured.
func BuildClaudeCmd(dir string) *exec.Cmd {
	cfg, _ := LoadConfig()

	var selected APIConfig
	if cfg != nil {
		for _, c := range cfg.Profiles {
			if c.Name == cfg.Default {
				selected = c
				break
			}
		}
	}

	SetEnvironmentVarsWithConfig(&selected)

	var args []string
	if ShouldSkipPermissionsWithConfig(&selected, cfg) {
		args = []string{"--dangerously-skip-permissions"}
	}

	cmd := exec.Command("claude", args...)
	cmd.Dir = dir
	return cmd
}

// ClaudeCmdSpec returns the command arguments and environment variables for launching
// Claude Code without modifying the current process environment.
func ClaudeCmdSpec() (args []string, env map[string]string) {
	cfg, _ := LoadConfig()

	var selected APIConfig
	if cfg != nil {
		for _, c := range cfg.Profiles {
			if c.Name == cfg.Default {
				selected = c
				break
			}
		}
	}

	env = GetEnvironmentVars(&selected)

	if ShouldSkipPermissionsWithConfig(&selected, cfg) {
		args = []string{"--dangerously-skip-permissions"}
	}

	return args, env
}

// GetTerminal returns the configured terminal emulator name.
// Returns empty string if not configured (uses platform default).
func GetTerminal() string {
	cfg, err := LoadConfig()
	if err != nil || cfg == nil {
		return ""
	}
	return cfg.Terminal
}

// SetTerminal saves the terminal emulator preference.
func SetTerminal(terminal string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.Terminal = terminal
	return SaveConfig(cfg)
}

// GetEditor returns the configured editor command.
func GetEditor() string {
	cfg, err := LoadConfig()
	if err != nil || cfg == nil {
		return ""
	}
	return cfg.Editor
}

// SetEditor saves the editor command to config.
func SetEditor(editor string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.Editor = editor
	return SaveConfig(cfg)
}

// TerminalOptions returns the list of known terminal emulator options.
func TerminalOptions() []string {
	return []string{"terminal", "iterm", "warp"}
}

// GetProjectsDir returns the configured projects directory, defaulting to ~/Projects.
func GetProjectsDir() string {
	cfg, err := LoadConfig()
	if err != nil || cfg == nil {
		return defaultProjectsDir()
	}
	if cfg.ProjectsDir != "" {
		return cfg.ProjectsDir
	}
	return defaultProjectsDir()
}

// SetProjectsDir sets the projects directory in config.
func SetProjectsDir(dir string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.ProjectsDir = dir
	return SaveConfig(cfg)
}

// GetAutoUpdate returns the configured auto-update mode.
// Defaults to "notify" if not set.
func GetAutoUpdate() string {
	cfg, err := LoadConfig()
	if err != nil {
		return "notify"
	}
	if cfg.AutoUpdate == "" {
		return "notify"
	}
	return cfg.AutoUpdate
}

// SetAutoUpdate sets the auto-update mode. Valid values: "notify", "silent", "off".
func SetAutoUpdate(mode string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.AutoUpdate = mode
	return SaveConfig(cfg)
}

// ProjectsDirOptions returns preset directory options for the projects dir setting.
func ProjectsDirOptions() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{"~/Projects", "~/Code", "~/Developer"}
	}
	return []string{
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "Developer"),
	}
}

func defaultProjectsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/Projects"
	}
	return filepath.Join(home, "Projects")
}

// GetDefaultEnvironmentVars 获取默认的环境变量提示
func GetDefaultEnvironmentVars() map[string]string {
	return map[string]string{
		"ANTHROPIC_BASE_URL":                "API endpoint URL (e.g., https://api.anthropic.com)",
		"ANTHROPIC_AUTH_TOKEN":              "Authentication token (e.g., sk-ant-...)",
		"ANTHROPIC_API_KEY":                 "API key for Claude SDK (interactive use)",
		"CLAUDE_CODE_API_KEY_HELPER_TTL_MS": "API key helper TTL in milliseconds",
		"HTTP_PROXY":                        "HTTP proxy server",
		"HTTPS_PROXY":                       "HTTPS proxy server",
		"NO_PROXY":                          "Domains and IPs to bypass proxy",
		"ANTHROPIC_MODEL":                   "Model name to use",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":     "Default Haiku model",
		"ANTHROPIC_DEFAULT_SONNET_MODEL":    "Default Sonnet model",
		"ANTHROPIC_DEFAULT_OPUS_MODEL":      "Default Opus model",
		"MAX_THINKING_TOKENS":               "Maximum thinking tokens for extended thinking",
	}
}

// GetDefaultBehavior 获取默认启动行为
func GetDefaultBehavior() string {
	cfg, err := LoadConfig()
	if err != nil {
		// 默认使用当前目录
		return "current"
	}

	if cfg.DefaultBehavior == "" {
		return "current"
	}

	// 验证值是否有效
	if cfg.DefaultBehavior == "current" || cfg.DefaultBehavior == "last" || cfg.DefaultBehavior == "home" {
		return cfg.DefaultBehavior
	}

	// 无效值，回退到默认
	return "current"
}

// ProjectInfo holds detailed information about a registered project.
type ProjectInfo struct {
	Name           string        `json:"name"`
	Path           string        `json:"path"`
	Remote         string        `json:"remote,omitempty"` // remote host name, empty = local
	Exists         bool          `json:"exists"`
	GitBranch      string        `json:"gitBranch,omitempty"`
	GitDirty       bool          `json:"gitDirty"`
	HasClaudeMD    bool          `json:"hasClaudeMd"`
	RecentBranches []string      `json:"recentBranches,omitempty"`
	Links          []ProjectLink `json:"links,omitempty"`
}

// GetProjectInfo aggregates project metadata including git status and file checks.
func GetProjectInfo(name, path string) ProjectInfo {
	return GetProjectInfoFromEntry(name, ProjectEntry{Path: path})
}

// GetProjectInfoFromEntry aggregates project metadata from a ProjectEntry.
func GetProjectInfoFromEntry(name string, entry ProjectEntry) ProjectInfo {
	info := ProjectInfo{
		Name:   name,
		Path:   entry.Path,
		Remote: entry.Remote,
		Links:  entry.Links,
	}

	// For remote projects, skip local filesystem checks
	if entry.Remote != "" {
		info.Exists = true // assume remote path exists
		return info
	}

	if _, err := os.Stat(entry.Path); err != nil {
		return info
	}
	info.Exists = true

	info.GitBranch = getGitBranch(entry.Path)
	info.GitDirty = isGitDirty(entry.Path)
	info.HasClaudeMD = hasClaudeMD(entry.Path)
	info.RecentBranches = getRecentGitBranches(entry.Path, 5)

	return info
}

func getGitBranch(dir string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func isGitDirty(dir string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func getRecentGitBranches(dir string, n int) []string {
	cmd := exec.Command("git", "branch", "--sort=-committerdate", "--format=%(refname:short)")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil
	}
	if len(lines) > n {
		lines = lines[:n]
	}
	return lines
}

func hasClaudeMD(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "CLAUDE.md")); err == nil {
		return true
	}
	return false
}

// AddRemote adds a remote host configuration.
func AddRemote(host RemoteHost) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	// Check for duplicate name
	for _, r := range cfg.Remotes {
		if r.Name == host.Name {
			return fmt.Errorf("remote %q already exists", host.Name)
		}
	}

	cfg.Remotes = append(cfg.Remotes, host)
	return SaveConfig(cfg)
}

// RemoveRemote removes a remote host by name.
func RemoveRemote(name string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	for i, r := range cfg.Remotes {
		if r.Name == name {
			cfg.Remotes = append(cfg.Remotes[:i], cfg.Remotes[i+1:]...)
			return SaveConfig(cfg)
		}
	}
	return nil
}

// GetRemote returns a remote host by name.
func GetRemote(name string) (*RemoteHost, bool) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, false
	}

	for _, r := range cfg.Remotes {
		if r.Name == name {
			return &r, true
		}
	}
	return nil, false
}

// ListRemotes returns all configured remote hosts.
func ListRemotes() ([]RemoteHost, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.Remotes, nil
}

// AddWebhook adds a webhook configuration.
func AddWebhook(webhook WebhookConfig) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	// Check for duplicate name (if name is provided)
	if webhook.Name != "" {
		for _, w := range cfg.Webhooks {
			if w.Name == webhook.Name {
				return fmt.Errorf("webhook %q already exists", webhook.Name)
			}
		}
	}

	// Default format to "slack" if not specified
	if webhook.Format == "" {
		webhook.Format = "slack"
	}

	cfg.Webhooks = append(cfg.Webhooks, webhook)
	return SaveConfig(cfg)
}

// RemoveWebhook removes a webhook by name or URL.
func RemoveWebhook(identifier string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	for i, w := range cfg.Webhooks {
		if w.Name == identifier || w.URL == identifier {
			cfg.Webhooks = append(cfg.Webhooks[:i], cfg.Webhooks[i+1:]...)
			return SaveConfig(cfg)
		}
	}
	return fmt.Errorf("webhook %q not found", identifier)
}

// GetWebhook returns a webhook by name or URL.
func GetWebhook(identifier string) (*WebhookConfig, bool) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, false
	}

	for _, w := range cfg.Webhooks {
		if w.Name == identifier || w.URL == identifier {
			return &w, true
		}
	}
	return nil, false
}

// ListWebhooks returns all configured webhooks.
func ListWebhooks() ([]WebhookConfig, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.Webhooks, nil
}
