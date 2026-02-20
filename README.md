# Codes CLI

[English](README.md) | [中文](README.zh-CN.md)

Environment configuration management, project management, and multi-agent collaboration tool for Claude Code. Switch API profiles instantly, manage project workspaces, and orchestrate autonomous agent teams to tackle complex tasks in parallel.

## Features

- **Profile Switching** — Manage multiple API configurations (Anthropic, proxies, custom endpoints) and switch instantly
- **Project Management** — Project aliases, workspace management, interactive TUI
- **Agent Teams** — Autonomous Claude agents collaborating with task dependencies, messaging, and auto-reporting
- **Workflow Templates** — YAML-based agent team templates for repeatable multi-agent pipelines
- **Cost Tracking** — Session-level API usage statistics by project and model
- **HTTP REST API** — Full REST API server (`codes serve`) for remote access, mobile clients, and WebSocket-based chat sessions
- **MCP Server** — 43 tools over stdio + SSE (served at `/mcp/` on same port as HTTP, no extra port needed)
- **Cross-Platform** — Linux, macOS, Windows (amd64 & arm64)

## Install

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/ourines/codes/main/install.sh | sh

# Windows (PowerShell)
irm https://raw.githubusercontent.com/ourines/codes/main/install.ps1 | iex

# From source (Go 1.24+)
git clone https://github.com/ourines/codes.git && cd codes && make build
```

Then run `codes init` to set up shell completion and PATH.

## Claude Code Integration

Add `codes` as an MCP server to let Claude Code manage agent teams, projects, and profiles directly.

**Project-level** (`.mcp.json` in project root):

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

**User-level** (`~/.claude/claude_code_config.json`):

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

Once configured, Claude Code gains access to 43 MCP tools:

| Category | Tools | Examples |
|----------|-------|---------|
| **Config** (10) | Projects, profiles, remotes | `list_projects`, `switch_profile`, `sync_remote` |
| **Agent** (25) | Teams, tasks, messages | `team_create`, `task_create`, `message_send` |
| **Stats** (4) | Usage tracking | `stats_summary`, `stats_by_project`, `stats_by_model` |
| **Workflow** (4) | Templates | `workflow_list`, `workflow_run`, `workflow_create` |

Usage in Claude Code:

```
You: Create a team to refactor the auth module

Claude: I'll set up a team with a coder and tester...
        [uses team_create, agent_add, task_create tools]

You: What's the status?

Claude: [uses team_status tool]
        The coder completed 2/3 tasks. Tester is waiting on task #3.
```

## Quick Start: Agent Teams

```bash
# Create a team with agents
codes agent team create myteam --workdir ~/Projects/myproject
codes agent add myteam coder --role "implementation" --model sonnet
codes agent add myteam tester --role "testing" --model sonnet

# Start agents and create tasks
codes agent start-all myteam
codes agent task create myteam "Implement login API" --assign coder --priority high
codes agent task create myteam "Write login tests" --assign tester --blocked-by 1

# Monitor and clean up
codes agent status myteam
codes agent stop-all myteam
```

### How It Works

Agents run as independent daemon processes, polling a shared file-based task queue every 3 seconds. Each agent executes tasks by spawning Claude CLI subprocesses and auto-reports results to the team.

All state lives in `~/.codes/teams/<name>/` as JSON files — no databases, no message brokers. Filesystem atomic renames guarantee safe concurrent access.

## Workflow Templates

Workflows are reusable YAML templates that define agent teams and tasks. Running a workflow creates a team, starts agents, and queues tasks — all in one command.

```bash
# List available workflows
codes workflow list

# Run a built-in workflow
codes workflow run pre-pr-check

# Create your own
codes workflow create my-pipeline
```

Example workflow YAML (`~/.codes/workflows/my-pipeline.yml`):

```yaml
name: my-pipeline
description: Build, test, and review
agents:
  - name: builder
    role: Build and compile the project
  - name: tester
    role: Run tests and report failures
  - name: reviewer
    role: Review code quality
tasks:
  - subject: Build project
    assign: builder
    prompt: Run the build and fix any compilation errors
  - subject: Run tests
    assign: tester
    prompt: Execute the test suite and report results
    blocked_by: [1]
  - subject: Code review
    assign: reviewer
    prompt: Review recent changes for quality issues
    blocked_by: [1]
```

Workflows can also be created programmatically via the `workflow_create` MCP tool.

## HTTP REST API Server

`codes serve` starts the full daemon — no flags needed. Everything runs on a **single port** (default `:3456`):

| Service | Address |
|---------|---------|
| HTTP REST API | `http://host:3456/` |
| MCP SSE | `http://host:3456/mcp/` |
| stdio MCP | Auto-detected (when stdin is a pipe, e.g. Claude Code MCP config) |
| Assistant scheduler | Background goroutine |

**First run** auto-generates and saves an auth token to `~/.codes/config.json`. All endpoints (except `/health`) require:

```
Authorization: Bearer <token>
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check (no auth) |
| `POST` | `/dispatch` | Dispatch task to agent team |
| `POST` | `/dispatch/simple` | Simplified single-task dispatch |
| `GET/POST` | `/sessions` | List / create chat sessions |
| `GET/DELETE` | `/sessions/{id}` | Get / delete session |
| `GET` | `/sessions/{id}/ws` | WebSocket stream (real-time I/O) |
| `POST` | `/sessions/{id}/message` | Send message to session |
| `POST` | `/sessions/{id}/interrupt` | Interrupt running session |
| `POST` | `/sessions/{id}/resume` | Resume paused session |
| `GET` | `/projects` | List projects |
| `GET` | `/projects/{name}` | Get project details |
| `GET` | `/profiles` | List profiles |
| `POST` | `/profiles/switch` | Switch active profile |
| `GET` | `/stats/summary` | Cost summary |
| `GET` | `/stats/projects` | Cost by project |
| `GET` | `/stats/models` | Cost by model |
| `POST` | `/stats/refresh` | Rebuild stats cache |
| `GET` | `/workflows` | List workflows |
| `GET/POST` | `/workflows/{name}` | Get / run workflow |
| `GET/POST` | `/teams` | List / create teams |
| `GET/DELETE` | `/teams/{name}` | Get / delete team |
| `GET` | `/teams/{name}/status` | Team dashboard |
| `GET/POST` | `/teams/{name}/tasks` | List / create tasks |
| `GET` | `/tasks/{id}` | Get task by ID |

### Configuration

Add to `~/.codes/config.json` to pin the bind address or pre-set tokens:

```json
{
  "httpBind": ":3456",
  "httpTokens": ["your-secret-token"]
}
```

## Commands

```
codes                                    # Launch TUI (when TTY detected)
codes init [--yes]                       # Install binary + shell completion
codes start <path|alias>                 # Launch Claude in directory (alias: s)
codes version / update                   # Version info / update Claude CLI
codes doctor                             # System diagnostics
codes serve                              # Start full daemon (HTTP :3456 + SSE MCP /mcp/ + scheduler)
```

### Profile Management (`codes profile`, alias: `pf`)

```bash
codes profile add                        # Add new profile interactively
codes profile select                     # Switch active profile
codes profile test [name]                # Test connectivity
codes profile list / remove <name>
```

### Project Aliases (`codes project`, alias: `p`)

```bash
codes project add [name] [path]          # Add project alias
codes project list / remove <name>
```

### Configuration (`codes config`, alias: `c`)

```bash
codes config get [key]                   # Show settings
codes config set <key> <value>           # Set value
codes config list <key>                  # List available values
codes config reset [key]                 # Reset to default
codes config export / import <file>      # Export/import configuration
```

| Key | Values | Description |
|-----|--------|-------------|
| `default-behavior` | `current`, `last`, `home` | Startup directory |
| `skip-permissions` | `true`, `false` | Skip permission prompts |
| `terminal` | `terminal`, `iterm`, `warp` | Terminal emulator |

### Agent Teams (`codes agent`, alias: `a`)

```bash
# Teams
codes agent team create <name> [--workdir <path>] [--description <text>]
codes agent team list / info <name> / delete <name>
codes agent status <name>                # Team dashboard

# Agents
codes agent add <team> <name> [--role <role>] [--model <model>] [--type worker|leader]
codes agent remove <team> <name>
codes agent start|stop <team> <name>
codes agent start-all|stop-all <team>

# Tasks
codes agent task create <team> <subject> [--assign <agent>] [--priority high|normal|low] [--blocked-by <ids>]
codes agent task list <team> [--status <status>] [--owner <agent>]
codes agent task get <team> <id> / cancel <team> <id>

# Messages
codes agent message send <team> <content> --from <agent> [--to <agent>]
codes agent message list <team> --agent <name>
```

### Workflow Templates (`codes workflow`, alias: `wf`)

```bash
codes workflow list                      # List all workflows
codes workflow run <name> [-d <dir>] [-m <model>] [-p <project>]
codes workflow create <name>             # Create template
codes workflow delete <name>
```

### Cost Tracking (`codes stats`, alias: `st`)

```bash
codes stats summary [period]             # Cost summary (today/week/month/all)
codes stats project [name]               # Cost by project
codes stats model                        # Cost by model
codes stats refresh                      # Force cache rebuild
```

### Remote Hosts (`codes remote`, alias: `r`)

```bash
codes remote add <name> <user@host>
codes remote list / status <name>
codes remote setup <name> / ssh <name>
```

## Configuration

Config file location: `~/.codes/config.json` (fallback: `./config.json`)

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
<summary>Supported environment variables</summary>

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_BASE_URL` | API endpoint URL |
| `ANTHROPIC_AUTH_TOKEN` | Authentication token |
| `ANTHROPIC_API_KEY` | API key (alternative auth) |
| `ANTHROPIC_MODEL` | Default model |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | Haiku model override |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | Sonnet model override |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | Opus model override |
| `MAX_THINKING_TOKENS` | Maximum thinking tokens |
| `HTTP_PROXY` / `HTTPS_PROXY` | Proxy settings |

</details>

## Development

```
codes/
├── cmd/codes/          # Entry point
├── internal/
│   ├── agent/          # Agent teams: daemon, runner, storage
│   ├── chatsession/    # Claude chat session lifecycle + WebSocket streaming
│   ├── commands/       # Cobra CLI commands
│   ├── config/         # Configuration management
│   ├── dispatch/       # Intent-based task dispatch to agent teams
│   ├── httpserver/     # HTTP REST API server (sessions, projects, stats, workflows)
│   ├── mcp/            # MCP server (43 tools, stdio transport)
│   ├── session/        # Terminal session manager
│   ├── stats/          # Cost tracking and aggregation
│   ├── remote/         # SSH remote management
│   ├── tui/            # Interactive TUI (bubbletea)
│   ├── ui/             # CLI output helpers
│   └── workflow/       # Workflow templates and orchestration
└── .github/workflows/  # CI/CD
```

```bash
make build    # Build binary
make test     # Run tests
go vet ./...  # Lint
```

## License

[MIT License](LICENSE)
