package assistant

import (
	"context"
	"fmt"
	"os"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"codes/internal/assistant/memory"
	"codes/internal/config"
)

const (
	defaultModel     = anthropic.ModelClaude3_5HaikuLatest
	defaultMaxTokens = 4096
)

// buildSystemPrompt generates the system prompt dynamically by injecting the
// user profile and a summary of recent memories.
func buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString("你是用户的个人 AI 助理，拥有记忆能力，可以帮助管理开发任务、记录偏好、设置提醒。\n")

	// -- User profile section --
	p, err := memory.LoadProfile()
	if err == nil && p != nil {
		hasField := p.Name != "" || p.Timezone != "" || p.Language != "" || p.DefaultProject != "" || p.Notes != ""
		if hasField {
			sb.WriteString("\n## 用户画像\n")
			if p.Name != "" {
				sb.WriteString("姓名: " + p.Name + "\n")
			}
			if p.Timezone != "" {
				sb.WriteString("时区: " + p.Timezone + "\n")
			}
			if p.Language != "" {
				sb.WriteString("语言偏好: " + p.Language + "\n")
			}
			if p.DefaultProject != "" {
				sb.WriteString("默认项目: " + p.DefaultProject + "\n")
			}
			if p.Notes != "" {
				sb.WriteString("备注: " + p.Notes + "\n")
			}
		}
	}

	// -- Memory summary section (up to 20 entities) --
	entities, _, err := memory.LoadGraph()
	if err == nil && len(entities) > 0 {
		limit := 20
		if len(entities) < limit {
			limit = len(entities)
		}
		sb.WriteString("\n## 记忆摘要（最近 20 条）\n")
		for _, e := range entities[:limit] {
			obs := strings.Join(e.Observations, "; ")
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", e.EntityType, e.Name, obs))
		}
	}

	// -- Usage guidelines --
	sb.WriteString(`
## 能力说明
- 查看用户已注册的项目列表
- 创建并运行编码任务（修 bug、新增功能、重构等）
- 查看正在运行的 agent 团队状态
- 将复杂需求拆解为并行任务
- 记忆用户偏好和项目信息（remember / recall / forget）
- 设置定时提醒（set_reminder / set_schedule / list_schedules / cancel_schedule）

## 使用指南
- 学到新的用户信息时，主动调用 remember 工具保存
- 用户问"我之前说过..."时，先调用 recall 搜索记忆
- 用户说"提醒我..."时，调用 set_reminder 或 set_schedule
- 如果不清楚用户指哪个项目，先调用 list_projects
- 派发任务后告知用户团队名称，以便后续查询进度

简洁回复。派发任务时，确认操作内容和目标项目。`)

	return sb.String()
}

// RunOptions configures a single assistant turn.
type RunOptions struct {
	SessionID string             // identifies the conversation (e.g. feishu chat_id, "default")
	Message   string             // user's message
	Model     anthropic.Model    // override model (optional)
}

// RunResult is the assistant's response.
type RunResult struct {
	Reply string
}

// Run processes one user message within a persistent session and returns the assistant's reply.
// The session is loaded from disk, updated, and saved back atomically.
func Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	if opts.Message == "" {
		return nil, fmt.Errorf("message is required")
	}
	if opts.SessionID == "" {
		opts.SessionID = "default"
	}

	// Resolve API credentials from active profile.
	apiKey, baseURL, err := resolveCredentials()
	if err != nil {
		return nil, fmt.Errorf("resolve credentials: %w", err)
	}

	// Build client.
	clientOpts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(clientOpts...)

	// Load session history.
	session, err := LoadSession(opts.SessionID)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}

	// Append the new user message.
	session.Messages = append(session.Messages,
		anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(opts.Message)),
	)

	// Build tools.
	tools, err := buildTools()
	if err != nil {
		return nil, fmt.Errorf("build tools: %w", err)
	}

	model := opts.Model
	if model == "" {
		model = defaultModel
	}

	// Run the tool loop to completion.
	runner := client.Beta.Messages.NewToolRunner(tools, anthropic.BetaToolRunnerParams{
		BetaMessageNewParams: anthropic.BetaMessageNewParams{
			Model:     model,
			MaxTokens: defaultMaxTokens,
			System: []anthropic.BetaTextBlockParam{
				{Text: buildSystemPrompt()},
			},
			Messages: session.Messages,
		},
	})

	msg, err := runner.RunToCompletion(ctx)
	if err != nil {
		return nil, fmt.Errorf("run assistant: %w", err)
	}

	// Extract reply text.
	reply := extractText(msg)

	// Persist the updated conversation (full history from runner).
	session.Messages = runner.Messages()
	if saveErr := session.Save(); saveErr != nil {
		// Non-fatal: log but don't fail the request.
		_ = saveErr
	}

	return &RunResult{Reply: reply}, nil
}

// extractText pulls all text blocks from the assistant message into a single string.
func extractText(msg *anthropic.BetaMessage) string {
	if msg == nil {
		return ""
	}
	var parts []string
	for _, block := range msg.Content {
		if b, ok := block.AsAny().(anthropic.BetaTextBlock); ok && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// resolveCredentials loads API key and base URL from the active profile.
func resolveCredentials() (apiKey, baseURL string, err error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", "", err
	}

	var active *config.APIConfig
	for i := range cfg.Profiles {
		if cfg.Profiles[i].Name == cfg.Default {
			active = &cfg.Profiles[i]
			break
		}
	}
	if active == nil && len(cfg.Profiles) > 0 {
		active = &cfg.Profiles[0]
	}
	if active == nil {
		// Fallback to environment variables when no profiles are configured.
		apiKey = os.Getenv("ANTHROPIC_AUTH_TOKEN")
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		baseURL = os.Getenv("ANTHROPIC_BASE_URL")
		if apiKey == "" {
			return "", "", fmt.Errorf("no API profiles configured and no ANTHROPIC_API_KEY/ANTHROPIC_AUTH_TOKEN env var found")
		}
		return apiKey, baseURL, nil
	}

	env := config.GetEnvironmentVars(active)
	apiKey = env["ANTHROPIC_AUTH_TOKEN"]
	if apiKey == "" {
		apiKey = env["ANTHROPIC_API_KEY"]
	}
	baseURL = env["ANTHROPIC_BASE_URL"]

	if apiKey == "" {
		return "", "", fmt.Errorf("no API key in active profile %q", active.Name)
	}
	return apiKey, baseURL, nil
}
