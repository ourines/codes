# Codes CLI

[English](README.md) | [中文](README.zh-CN.md)

Environment configuration management, project management, and multi-agent collaboration tool for Claude Code. Switch API profiles instantly, manage project workspaces, and orchestrate autonomous agent teams to tackle complex tasks in parallel.

## Features

- **Profile Switching** — Manage multiple API configurations (Anthropic, proxies, custom endpoints) and switch instantly
- **Project Management** — Project aliases, workspace management, interactive TUI
- **Agent Teams** — Autonomous Claude agents collaborating with task dependencies, messaging, and auto-reporting
- **MCP Server** — 29 tools integrated into Claude Code for managing everything from conversations
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

Once configured, Claude Code gains access to 29 MCP tools:

| Category | Tools | Examples |
|----------|-------|---------|
| **Config** (10) | Projects, profiles, remotes | `list_projects`, `switch_profile`, `sync_remote` |
| **Agent** (19) | Teams, tasks, messages | `team_create`, `task_create`, `message_send` |

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

## Commands

```
codes                                    # Launch TUI (when TTY detected)
codes init [--yes]                       # Install binary + shell completion
codes start <path|alias>                 # Launch Claude in directory (alias: s)
codes version / update                   # Version info / update Claude CLI
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
│   ├── mcp/            # MCP server (29 tools, stdio transport)
│   ├── tui/            # Interactive TUI (bubbletea)
│   ├── commands/       # Cobra CLI commands
│   ├── config/         # Configuration management
│   ├── session/        # Terminal session manager
│   ├── remote/         # SSH remote management
│   └── ui/             # CLI output helpers
└── .github/workflows/  # CI/CD
```

```bash
make build    # Build binary
make test     # Run tests
go vet ./...  # Lint
```

## License

[MIT License](LICENSE)
