# Codes CLI

[English](README.md) | [中文](README.zh-CN.md)

Claude Code 的环境配置管理、项目管理与多 Agent 协作工具。一键切换 API profile，管理多个项目工作区，并通过自治 Agent 团队并行完成复杂任务。

## 功能特性

- **Profile 切换** — 多套 API 环境配置（Anthropic、代理、自定义端点）一键切换
- **项目管理** — 项目别名、工作目录管理，TUI 可视化操作
- **Agent 团队** — 多个 Claude Agent 自治协作，任务依赖、消息传递、自动汇报
- **Workflow 模板** — YAML 定义的 Agent 团队模板，一键启动可复用的多 Agent 流水线
- **成本追踪** — 按项目、模型维度的 API 用量统计
- **HTTP REST API** — 内置 REST API Server（`codes serve`），支持远程访问、移动客户端和 WebSocket 实时对话
- **MCP Server** — 43 个工具集成到 Claude Code，直接在对话中管理一切
- **跨平台** — Linux, macOS, Windows (amd64 & arm64)

## 安装

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/ourines/codes/main/install.sh | sh

# Windows (PowerShell)
irm https://raw.githubusercontent.com/ourines/codes/main/install.ps1 | iex

# 从源码构建 (Go 1.24+)
git clone https://github.com/ourines/codes.git && cd codes && make build
```

安装后运行 `codes init` 设置 shell 补全和 PATH。

## Claude Code 集成

将 `codes` 添加为 MCP server，让 Claude Code 直接管理 Agent 团队、项目和 Profile。

**项目级配置**（项目根目录 `.mcp.json`）：

```json
{
  "mcpServers": {
    "codes": {
      "command": "codes",
      "args": ["serve"]
    }
  }
}
```

**用户级配置**（`~/.claude/claude_code_config.json`）：

```json
{
  "mcpServers": {
    "codes": {
      "command": "codes",
      "args": ["serve"]
    }
  }
}
```

配置完成后，Claude Code 即可使用 43 个 MCP 工具：

| 分类 | 工具 | 示例 |
|------|------|------|
| **配置管理** (10) | 项目、Profile、远程主机 | `list_projects`、`switch_profile`、`sync_remote` |
| **Agent** (25) | 团队、任务、消息 | `team_create`、`task_create`、`message_send` |
| **统计** (4) | 用量追踪 | `stats_summary`、`stats_by_project`、`stats_by_model` |
| **Workflow** (4) | 模板 | `workflow_list`、`workflow_run`、`workflow_create` |

在 Claude Code 中使用：

```
你: 创建一个团队来重构认证模块

Claude: 我来组建一个包含 coder 和 tester 的团队...
        [调用 team_create, agent_add, task_create 工具]

你: 进度如何？

Claude: [调用 team_status 工具]
        coder 已完成 2/3 个任务。tester 正在等待任务 #3 完成。
```

## 快速开始：Agent 团队

```bash
# 创建团队和 Agent
codes agent team create myteam --workdir ~/Projects/myproject
codes agent add myteam coder --role "implementation" --model sonnet
codes agent add myteam tester --role "testing" --model sonnet

# 启动 Agent 并创建任务
codes agent start-all myteam
codes agent task create myteam "Implement login API" --assign coder --priority high
codes agent task create myteam "Write login tests" --assign tester --blocked-by 1

# 查看状态和清理
codes agent status myteam
codes agent stop-all myteam
```

### 工作原理

Agent 以独立守护进程运行，每 3 秒轮询共享的文件任务队列。每个 Agent 通过启动 Claude CLI 子进程执行任务，并自动向团队汇报结果。

所有状态以 JSON 文件存储在 `~/.codes/teams/<name>/` 下 — 无需数据库或消息中间件。文件系统原子重命名保证并发安全。

## Workflow 模板

Workflow 是可复用的 YAML 模板，定义 Agent 团队和任务。运行 workflow 会自动创建团队、启动 Agent、提交任务 — 一条命令搞定。

```bash
# 列出可用 workflow
codes workflow list

# 运行内置 workflow
codes workflow run pre-pr-check

# 创建自定义 workflow
codes workflow create my-pipeline
```

Workflow YAML 示例（`~/.codes/workflows/my-pipeline.yml`）：

```yaml
name: my-pipeline
description: 构建、测试、审查
agents:
  - name: builder
    role: 构建编译项目
  - name: tester
    role: 运行测试并报告失败
  - name: reviewer
    role: 审查代码质量
tasks:
  - subject: 构建项目
    assign: builder
    prompt: 运行构建并修复编译错误
  - subject: 运行测试
    assign: tester
    prompt: 执行测试套件并报告结果
    blocked_by: [1]
  - subject: 代码审查
    assign: reviewer
    prompt: 审查最近的代码变更
    blocked_by: [1]
```

也可通过 `workflow_create` MCP 工具在对话中创建 workflow。

## HTTP REST API Server

`codes serve` 启动 REST API Server（默认 `:3456`），通过 HTTP 暴露所有 `codes` 功能，适用于 iOS/移动端 App 和远程自动化。

**首次运行**会自动生成 Auth Token 并保存到 `~/.codes/config.json`。所有端点（`/health` 除外）需携带：

```
Authorization: Bearer <token>
```

### 端点列表

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/health` | 健康检查（无需认证） |
| `POST` | `/dispatch` | 分发任务到 Agent 团队 |
| `POST` | `/dispatch/simple` | 简化版单任务分发 |
| `GET/POST` | `/sessions` | 列出 / 创建对话 Session |
| `GET/DELETE` | `/sessions/{id}` | 获取 / 删除 Session |
| `GET` | `/sessions/{id}/ws` | WebSocket 流（实时 I/O） |
| `POST` | `/sessions/{id}/message` | 向 Session 发送消息 |
| `POST` | `/sessions/{id}/interrupt` | 中断正在运行的 Session |
| `POST` | `/sessions/{id}/resume` | 恢复暂停的 Session |
| `GET` | `/projects` | 列出项目 |
| `GET` | `/projects/{name}` | 获取项目详情 |
| `GET` | `/profiles` | 列出 Profile |
| `POST` | `/profiles/switch` | 切换活跃 Profile |
| `GET` | `/stats/summary` | 费用概要 |
| `GET` | `/stats/projects` | 按项目统计费用 |
| `GET` | `/stats/models` | 按模型统计费用 |
| `POST` | `/stats/refresh` | 重建统计缓存 |
| `GET` | `/workflows` | 列出 Workflow |
| `GET/POST` | `/workflows/{name}` | 获取 / 运行 Workflow |
| `GET/POST` | `/teams` | 列出 / 创建团队 |
| `GET/DELETE` | `/teams/{name}` | 获取 / 删除团队 |
| `GET` | `/teams/{name}/status` | 团队仪表盘 |
| `GET/POST` | `/teams/{name}/tasks` | 列出 / 创建任务 |
| `GET` | `/tasks/{id}` | 按 ID 获取任务 |

### 配置

在 `~/.codes/config.json` 中可预设监听地址和 Token：

```json
{
  "httpBind": ":3456",
  "httpTokens": ["your-secret-token"]
}
```

## 命令参考

```
codes                                    # 启动 TUI（检测到 TTY 时）
codes init [--yes]                       # 安装二进制文件 + shell 补全
codes start <路径|别名>                   # 在指定目录启动 Claude（别名: s）
codes version / update                   # 版本信息 / 更新 Claude CLI
codes doctor                             # 系统诊断
codes serve [--addr :3456]              # 启动 HTTP REST API Server
```

### Profile 管理 (`codes profile`，别名: `pf`)

```bash
codes profile add                        # 交互式添加 Profile
codes profile select                     # 切换当前 Profile
codes profile test [name]                # 测试连接
codes profile list / remove <name>
```

### 项目别名 (`codes project`，别名: `p`)

```bash
codes project add [name] [path]          # 添加项目别名
codes project list / remove <name>
```

### 配置 (`codes config`，别名: `c`)

```bash
codes config get [key]                   # 查看配置
codes config set <key> <value>           # 设置值
codes config list <key>                  # 列出可选值
codes config reset [key]                 # 重置为默认
codes config export / import <file>      # 导出/导入配置
```

| 配置项 | 可选值 | 说明 |
|--------|--------|------|
| `default-behavior` | `current`、`last`、`home` | 启动目录 |
| `skip-permissions` | `true`、`false` | 跳过权限确认 |
| `terminal` | `terminal`、`iterm`、`warp` | 终端模拟器 |

### Agent 团队 (`codes agent`，别名: `a`)

```bash
# 团队
codes agent team create <name> [--workdir <路径>] [--description <描述>]
codes agent team list / info <name> / delete <name>
codes agent status <name>                # 团队仪表盘

# Agent
codes agent add <team> <name> [--role <角色>] [--model <模型>] [--type worker|leader]
codes agent remove <team> <name>
codes agent start|stop <team> <name>
codes agent start-all|stop-all <team>

# 任务
codes agent task create <team> <主题> [--assign <agent>] [--priority high|normal|low] [--blocked-by <ids>]
codes agent task list <team> [--status <状态>] [--owner <agent>]
codes agent task get <team> <id> / cancel <team> <id>

# 消息
codes agent message send <team> <内容> --from <agent> [--to <agent>]
codes agent message list <team> --agent <name>
```

### Workflow 模板 (`codes workflow`，别名: `wf`)

```bash
codes workflow list                      # 列出所有 workflow
codes workflow run <name> [-d <目录>] [-m <模型>] [-p <项目>]
codes workflow create <name>             # 创建模板
codes workflow delete <name>
```

### 成本追踪 (`codes stats`，别名: `st`)

```bash
codes stats summary [period]             # 成本概要 (today/week/month/all)
codes stats project [name]               # 按项目统计
codes stats model                        # 按模型统计
codes stats refresh                      # 强制刷新缓存
```

### 远程主机 (`codes remote`，别名: `r`)

```bash
codes remote add <name> <user@host>
codes remote list / status <name>
codes remote setup <name> / ssh <name>
```

## 配置文件

配置文件位置：`~/.codes/config.json`（回退：`./config.json`）

```json
{
  "profiles": [
    {
      "name": "work",
      "env": {
        "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
        "ANTHROPIC_AUTH_TOKEN": "sk-ant-xxxxx"
      }
    }
  ],
  "default": "work",
  "defaultBehavior": "current",
  "terminal": "terminal",
  "projects": { "my-project": "/path/to/project" }
}
```

<details>
<summary>支持的环境变量</summary>

| 变量 | 说明 |
|------|------|
| `ANTHROPIC_BASE_URL` | API 端点地址 |
| `ANTHROPIC_AUTH_TOKEN` | 认证 Token |
| `ANTHROPIC_API_KEY` | API Key（替代认证方式） |
| `ANTHROPIC_MODEL` | 默认模型 |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | Haiku 模型覆盖 |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | Sonnet 模型覆盖 |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | Opus 模型覆盖 |
| `MAX_THINKING_TOKENS` | 最大思考 Token 数 |
| `HTTP_PROXY` / `HTTPS_PROXY` | 代理设置 |

</details>

## 开发

```
codes/
├── cmd/codes/          # 入口
├── internal/
│   ├── agent/          # Agent 团队：守护进程、任务执行、存储
│   ├── chatsession/    # Claude 对话 Session 生命周期 + WebSocket 流
│   ├── commands/       # Cobra CLI 命令
│   ├── config/         # 配置管理
│   ├── dispatch/       # 意图驱动的任务分发到 Agent 团队
│   ├── httpserver/     # HTTP REST API Server（Session、项目、统计、Workflow）
│   ├── mcp/            # MCP Server（43 工具，stdio 传输）
│   ├── session/        # 终端会话管理
│   ├── stats/          # 成本追踪与聚合
│   ├── remote/         # SSH 远程管理
│   ├── tui/            # 交互式 TUI（bubbletea）
│   ├── ui/             # CLI 输出工具
│   └── workflow/       # Workflow 模板与编排
└── .github/workflows/  # CI/CD
```

```bash
make build    # 构建
make test     # 测试
go vet ./...  # 代码检查
```

## 许可证

[MIT License](LICENSE)
