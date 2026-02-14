# Codes CLI

A powerful CLI tool for orchestrating autonomous AI agent teams. Create multi-agent workflows where Claude instances collaborate through file-based task queues and message passing—no complex infrastructure required.

## Features

- **Agent Team System**: Coordinate multiple autonomous Claude agents working on distributed tasks with message passing, task dependencies, and auto-reporting
- **File-Based Coordination**: JSON-based task queues and message inboxes—works across platforms without PTY or complex IPC
- **MCP Server Integration**: 29 MCP tools (10 config + 19 agent) for seamless Claude Code integration
- **Interactive TUI**: Full terminal UI for managing projects, profiles, remotes, and settings
- **Multi-Profile Management**: Switch between API providers (Anthropic, proxies, custom endpoints)
- **Session Manager**: Launch Claude in separate terminal windows with multi-instance support
- **Smart Directory Launch**: Remember last working directory and support project aliases
- **Cross-Platform**: Support for Linux, macOS, and Windows (amd64 & arm64)
- **JSON Output**: Machine-readable output via `--json` flag for scripting
- **Shell Completion**: Auto-completion for bash, zsh, fish, and powershell

## Quick Install

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/ourines/codes/main/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/ourines/codes/main/install.ps1 | iex
```

## Quick Start: Agent Team Workflow

```bash
# 1. Create a team
codes agent team create myteam --workdir ~/Projects/myproject

# 2. Add agents
codes agent add myteam coder --role "code implementation" --model sonnet
codes agent add myteam tester --role "test validation" --model sonnet
codes agent add myteam lead --role "team coordinator" --model opus

# 3. Start all agents
codes agent start-all myteam

# 4. Create tasks with dependencies
codes agent task create myteam "Implement login API" \
  --assign coder --priority high
codes agent task create myteam "Write tests for login" \
  --assign tester --blocked-by 1

# 5. Monitor progress
codes agent status myteam

# 6. Check team status
codes agent status myteam

# 7. Clean up
codes agent stop-all myteam
```

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/ourines/codes/releases).

```bash
# Download and extract (example for linux-amd64)
curl -L https://github.com/ourines/codes/releases/latest/download/codes-linux-amd64 -o codes
chmod +x codes

# Install to system path and set up shell completion
./codes init
```

### Build from Source

Requirements:
- Go 1.24 or later
- npm (required for Claude CLI installation)

```bash
git clone https://github.com/ourines/codes.git
cd codes
make build
./codes init
```

## Usage

### Agent Team System

The agent team system is the core feature of `codes`, enabling multi-agent collaboration through autonomous Claude daemons that execute tasks and communicate via message passing.

#### Architecture

```
┌──────────────────┐
│  External AI     │  (Claude Code, custom tools, scripts)
│  (MCP Client)    │
└────────┬─────────┘
         │ MCP Tools (27)
         ▼
┌──────────────────┐
│  codes serve     │  MCP Server (stdio transport)
│  (MCP Server)    │  - 10 config tools (projects, profiles, remotes)
└────────┬─────────┘  - 17 agent tools (teams, tasks, messages)
         │
         │ Spawns & manages
         ▼
┌──────────────────────────────────────────────┐
│  Agent Daemons (independent processes)       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐     │
│  │ coder    │ │ tester   │ │ lead     │     │
│  │ daemon   │ │ daemon   │ │ daemon   │     │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘     │
└───────┼────────────┼────────────┼────────────┘
        │            │            │
        │ Executes   │ Executes   │ Executes
        ▼            ▼            ▼
┌────────────────────────���─────────────────────┐
│  Claude CLI subprocesses (task execution)    │
│  ┌──────────────┐ ┌──────────────┐           │
│  │ claude -p    │ │ claude -p    │ ...       │
│  │ --session-id │ │ --session-id │           │
│  │ --json       │ │ --json       │           │
│  └──────────────┘ └──────────────┘           │
└────────────────────────────────��──────────────┘
        │                    │
        ▼                    ▼
┌──────────────────────────────────────────────┐
│  File-Based Storage (~/.codes/teams/)        │
│  ├── config.json      (team config)          │
│  ├── tasks/*.json     (task queue)           │
│  ├── messages/*.json  (message inboxes)      │
│  └── agents/*.json    (agent state/PIDs)     │
└───────────────────────────────────────────────┘
```

#### Key Concepts

- **Team**: A workspace with shared task queue and message bus. All state stored in `~/.codes/teams/<team-name>/`
- **Agent**: An autonomous daemon (independent process) that polls for tasks, executes them via Claude CLI subprocesses, and auto-reports results
- **Task**: A unit of work with status tracking (`pending` → `assigned` → `running` → `completed`/`failed`), dependencies (`blockedBy`), and priority
- **Message**: Communication between agents. Types: `chat` (direct messages), `task_completed`/`task_failed` (auto-reports), `system` (control signals)
- **File-Based Coordination**: Tasks and messages stored as JSON files with atomic writes (temp + rename). No databases, no PTY—just files

#### Why File-Based?

- **Cross-platform**: Works anywhere with a filesystem (Linux, macOS, Windows, remote SSH)
- **No infrastructure**: No databases, message brokers, or network services
- **Debuggable**: Inspect state with `cat ~/.codes/teams/myteam/tasks/1.json`
- **Atomic operations**: Filesystem rename guarantees atomicity for task claims and message delivery
- **Survives restarts**: Agent daemons can crash and restart without losing state

#### Agent Daemon Lifecycle

Each agent runs as an independent process (`codes agent run <team> <agent>`), polling every 3 seconds:

1. **Check stop signal**: Read messages for `__stop__` command
2. **Process messages**: Chat messages routed to Claude subprocess, responses sent back to sender
3. **Execute tasks**: Auto-claim unassigned tasks or execute assigned tasks via Claude subprocess

State tracked in `~/.codes/teams/<team>/agents/<name>.json` with PID, status (`idle`/`running`/`stopping`/`stopped`), and persistent session ID.

#### Task Execution Flow

1. Agent finds next task (assigned or auto-claims pending)
2. Spawns Claude CLI subprocess: `claude -p "<prompt>" --output-format json --session-id <id>`
3. Parses JSON output into `ClaudeResult` (result, error, cost, duration)
4. Updates task status (`completed`/`failed`) and broadcasts report to team

#### MCP Integration Example

External AI tools (like Claude Code) can orchestrate agent teams via MCP:

```json
// ~/.claude/claude_desktop_config.json
{
  "mcpServers": {
    "codes": {
      "command": "codes",
      "args": ["serve"]
    }
  }
}
```

Then in Claude Code:

```
User: Create a team to refactor the auth module

Claude: I'll create a team and coordinate the work...
<uses team_create, agent_add, task_create MCP tools>

User: What's the status?

Claude: Let me check...
<uses team_status MCP tool>
The coder agent has completed 2 of 3 tasks. The tester is waiting on task #3 to finish.
```

#### CLI Examples

**Team Management:**

```bash
# Create team
codes agent team create myteam --description "API refactor" --workdir ~/Projects/api

# List teams
codes agent team list

# Show team dashboard
codes agent status myteam

# Delete team
codes agent team delete myteam
```

**Agent Management:**

```bash
# Add agents
codes agent add myteam coder --role "implementation" --model sonnet --type worker
codes agent add myteam lead --role "coordinator" --model opus --type leader

# Start/stop individual agents
codes agent start myteam coder
codes agent stop myteam coder

# Start/stop all agents
codes agent start-all myteam
codes agent stop-all myteam

# List agents with status (via team info or status command)
codes agent team info myteam  # shows all agents with their config
codes agent status myteam     # shows agents with live status
```

**Task Management:**

```bash
# Create tasks
codes agent task create myteam "Refactor auth handler" \
  --description "Extract validation logic to separate module" \
  --assign coder \
  --priority high

codes agent task create myteam "Update tests" \
  --assign tester \
  --blocked-by 1

# List tasks
codes agent task list myteam                   # all tasks
codes agent task list myteam --status running  # filter by status
codes agent task list myteam --owner coder     # filter by owner

# Get task details
codes agent task get myteam 1

# Cancel a task
codes agent task cancel myteam 1
```

**Messaging:**

```bash
# Send direct message
codes agent message send myteam "Please prioritize task #2" \
  --from lead --to coder

# Broadcast to all agents
codes agent message send myteam "All hands meeting at 3pm" --from lead

# List messages
codes agent message list myteam --agent coder                  # all messages for agent
```

#### File Storage Structure

```
~/.codes/teams/myteam/
├── config.json              # Team configuration
│   {
│     "name": "myteam",
│     "description": "API refactor",
│     "workDir": "/path/to/project",
│     "members": [
│       {"name": "coder", "role": "implementation", "model": "sonnet", "type": "worker"},
│       {"name": "tester", "role": "validation", "model": "sonnet", "type": "worker"}
│     ]
│   }
├── tasks/                   # Task queue (one file per task)
│   ├── 1.json
│   │   {
│   │     "id": 1,
│   │     "subject": "Refactor auth handler",
│   │     "status": "completed",
│   │     "owner": "coder",
│   │     "sessionId": "abc123",
│   │     "result": "Extracted validation to auth/validate.go"
│   │   }
│   └── 2.json
├── messages/                # Message inboxes (one directory per agent)
│   ├── coder/
│   │   └── msg-xyz.json
│   ├── tester/
│   └── lead/
└── agents/                  # Agent state (PID, status, current task)
    ├── coder.json
    │   {
    │     "name": "coder",
    │     "team": "myteam",
    │     "pid": 12345,
    │     "status": "running",
    │     "currentTask": 2,
    │     "sessionId": "persistent-session-123"
    │   }
    └── tester.json
```

### Interactive TUI

Running `codes` without arguments in a terminal launches the interactive TUI:

```bash
codes
```

The TUI has three tabs (cycle with `Tab`):

| Tab | Description | Key Bindings |
|-----|-------------|--------------|
| **Projects** | Manage project aliases, launch sessions | `a` add, `d` delete, `Enter` open session, `→` sessions panel, `k` kill, `t` cycle terminal |
| **Profiles** | Manage API profiles, switch default | `a` add profile, `Enter` set as default |
| **Settings** | Configure terminal, behavior, permissions | `↑↓` navigate, `Enter`/`Space` cycle value |

**Settings tab options:**

| Setting | Values | Description |
|---------|--------|-------------|
| Terminal | terminal / iterm / warp | Terminal emulator for sessions |
| Default Behavior | current / last / home | Where Claude starts without arguments |
| Skip Permissions | off / on | Global `--dangerously-skip-permissions` |
| Config File | *(read-only)* | Shows config file path |

### Session Manager

From the Projects tab, press `Enter` on a project to launch Claude in a new terminal window. Each project supports multiple concurrent sessions.

- Sessions open in the configured terminal (Terminal.app, iTerm2, or Warp on macOS)
- Press `→` to view running sessions for the selected project
- Press `k` to kill sessions
- Session status auto-refreshes every 3 seconds

### MCP Server

Start codes as an MCP server for integration with Claude Code:

```bash
codes serve
```

**MCP Tools (29 total):**

**Config Tools (10):**

| Tool | Description |
|------|-------------|
| `list_projects` | List all project aliases with git status |
| `add_project` | Add a new project alias |
| `remove_project` | Remove a project alias |
| `get_project_info` | Get detailed project info (git branch, dirty status) |
| `list_profiles` | List all API profiles with status |
| `switch_profile` | Switch the default API profile |
| `list_remotes` | List all configured remote SSH hosts |
| `add_remote` | Add a new remote SSH host |
| `remove_remote` | Remove a remote SSH host |
| `sync_remote` | Sync profiles to a remote host |

**Agent Tools (19):**

| Tool | Description |
|------|-------------|
| `team_create` | Create a new team workspace |
| `team_delete` | Delete a team and all data |
| `team_list` | List all teams |
| `team_get` | Get team config and agent statuses |
| `team_status` | Get team dashboard (agents, tasks, completions) |
| `team_start_all` | Start all agents in a team |
| `team_stop_all` | Stop all agents in a team |
| `agent_add` | Register a new agent in a team |
| `agent_remove` | Remove an agent from a team |
| `agent_list` | List agents with live status |
| `agent_start` | Start an agent daemon |
| `agent_stop` | Stop a running agent daemon |
| `task_create` | Create a task with optional assignment |
| `task_update` | Update task status, owner, result, etc. |
| `task_list` | List tasks with filters |
| `task_get` | Get task details |
| `message_send` | Send message between agents or broadcast |
| `message_list` | List messages for an agent (with type/unread filters) |
| `message_mark_read` | Mark a message as read |

**Claude Code MCP config** (`~/.claude/claude_desktop_config.json`):

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

## Commands

### `codes init`

Initialize the CLI: install binary, set up shell completion, run health checks.

```bash
codes init
```

### `codes profile` (alias: `pf`)

Manage API profiles.

```bash
codes profile add              # interactively add a new profile
codes profile select           # select active profile
codes profile test             # test all profiles
codes profile test work        # test specific profile
codes profile list             # list all profiles
codes profile remove work      # remove a profile
codes pf list                  # alias shorthand
```

### `codes start [path]` (alias: `s`)

Start Claude in a specific directory or project alias.

```bash
codes start .              # current directory
codes start /path/to/dir   # specific path
codes start my-project     # project alias
codes s my-project         # alias shorthand
```

### `codes project` (alias: `p`)

Manage project aliases.

```bash
codes project add                          # add cwd as project (auto-name)
codes project add /path/to/my-app          # add path (auto-name from dir)
codes project add my-app /path/to/my-app   # add with explicit name
codes project list
codes project remove my-app
codes p add .                              # alias shorthand
```

### `codes config` (alias: `c`)

Manage CLI configuration. Replaces the old `defaultbehavior`, `skippermissions`, and `terminal` commands.

```bash
codes config get                              # show all settings
codes config get default-behavior             # show specific setting
codes config set default-behavior last        # current | last | home
codes config set skip-permissions true        # true | false
codes config set terminal iterm               # terminal emulator
codes config list terminal                    # list available values
codes config reset skip-permissions           # reset to default
codes config reset                            # reset all settings
codes c set terminal warp                     # alias shorthand
```

### `codes update`

Update Claude CLI to a specific version.

```bash
codes update
```

### `codes serve`

Start MCP server over stdio.

```bash
codes serve
```

### `codes remote` (alias: `r`)

Manage remote SSH hosts for running Claude Code remotely.

```bash
codes remote add myhost user@example.com
codes remote list
codes remote status myhost
codes remote setup myhost
codes remote ssh myhost
codes r list                    # alias shorthand
```

### `codes agent` (alias: `a`)

Manage agent teams and autonomous task execution.

#### Team Management

```bash
codes agent team create <name>                # create a team
codes agent team create demo \
  --description "Demo team" \
  --workdir ~/Projects/demo                   # with options

codes agent team list                         # list all teams
codes agent team info demo                    # show team details
codes agent team delete demo                  # delete a team
codes agent status demo                       # team dashboard
```

#### Agent Management

```bash
codes agent add <team> <name>                 # add agent to team
codes agent add demo coder \
  --role "code implementation" \
  --model sonnet \
  --type worker                               # with options

codes agent list demo                         # list agents with status
codes agent remove demo coder                 # remove agent from team
codes agent start demo coder                  # start agent daemon
codes agent stop demo coder                   # stop agent daemon
codes agent start-all demo                    # start all agents
codes agent stop-all demo                     # stop all agents
```

#### Task Management

```bash
codes agent task create <team> <subject>      # create task
codes agent task create demo "Fix login bug" \
  --description "Details here" \
  --assign coder \
  --priority high \
  --blocked-by 1,2                            # with options

codes agent task list demo                    # list all tasks
codes agent task list demo \
  --status running \
  --owner coder                               # with filters

codes agent task get demo 1                   # get task details
codes agent task cancel demo 1                # cancel a task
```

#### Messaging

```bash
codes agent message send <team> <content>     # send message
codes agent message send demo "Status update" \
  --from lead \
  --to coder                                  # direct message

codes agent message send demo "All hands meeting" \
  --from lead                                 # broadcast (no --to)

codes agent message list demo \
  --agent coder                               # list messages for agent
```

### `codes version`

Display the current version.

```bash
codes version
```

## Configuration

### File Location

The tool searches for `config.json` in order:

1. Current working directory: `./config.json`
2. User home directory: `~/.codes/config.json`

### Format

```json
{
  "profiles": [
    {
      "name": "work",
      "env": {
        "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
        "ANTHROPIC_AUTH_TOKEN": "sk-ant-xxxxx",
        "ANTHROPIC_MODEL": "claude-sonnet-4-20250514"
      },
      "skipPermissions": false,
      "status": "active"
    }
  ],
  "default": "work",
  "skipPermissions": false,
  "defaultBehavior": "current",
  "terminal": "terminal",
  "projects": {
    "my-project": "/path/to/project"
  },
  "lastWorkDir": "/path/to/last/directory"
}
```

> **Backward compatibility**: Old config files using `"configs"` instead of `"profiles"` are automatically migrated on load.

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `profiles` | array | API profile configurations |
| `default` | string | Name of the default profile |
| `skipPermissions` | bool | Global `--dangerously-skip-permissions` flag |
| `defaultBehavior` | string | Startup directory: `current`, `last`, or `home` |
| `terminal` | string | Terminal emulator: `terminal`, `iterm`, `warp`, or custom |
| `projects` | object | Project name → directory path mappings |
| `lastWorkDir` | string | Last working directory (auto-saved) |

### Profile Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique profile identifier |
| `env` | object | Environment variables for Claude CLI |
| `skipPermissions` | bool | Per-profile override (optional) |
| `status` | string | `active`, `inactive`, or `unknown` |

### Supported Environment Variables

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
| `NO_PROXY` | Proxy bypass list |

## Roadmap

### Agent System Enhancements

- **Multi-Model Support**: Not just Claude—support for other LLM CLIs (OpenAI, Gemini, local models via Ollama)
  - Model-agnostic task execution via unified CLI interface
  - Per-agent model configuration (mix Claude + GPT-4 + local models in one team)

- **Dashboard & Watch Mode**: Real-time team monitoring
  - `codes agent status --watch` with auto-refresh
  - Live task progress tracking
  - Agent health indicators (CPU, memory, active sessions)

- **Agent-to-Agent Direct Communication**: Beyond message passing
  - Direct Claude session sharing (one agent resumes another's context)
  - Shared memory/knowledge base (team-wide context)
  - Peer collaboration patterns (code review, pair programming)

- **Task Dependency Graph Visualization**: Understand complex workflows
  - `codes agent task graph myteam --output mermaid`
  - Critical path analysis
  - Blocked task diagnostics

- **Advanced Coordination Patterns**:
  - Distributed file locks for cross-machine teams (NFS, shared volumes)
  - Remote agent deployment (SSH into VPS, spawn agents there)
  - Task scheduling (cron-like, time-based triggers)
  - Retry policies and exponential backoff

### TUI & UX Improvements

- Team management tab in TUI (visual team dashboard)
- Interactive task creation wizard
- Log streaming from agent daemons

### Testing & Reliability

- Integration tests for agent lifecycle
- Chaos testing (kill agents mid-task, verify recovery)
- Benchmark task throughput and latency

## Development

### Project Structure

```
codes/
├── cmd/codes/              # Main entry point
├── internal/
│   ├── agent/              # Agent team system (daemon, runner, types, storage)
│   ├── commands/           # CLI command implementations
│   ├── config/             # Configuration management
│   ├── mcp/                # MCP server (stdio transport, 27 tools)
│   ├── output/             # JSON output mode
│   ├── remote/             # SSH remote management
│   ├── session/            # Terminal session manager
│   ├── tui/                # Interactive TUI (bubbletea)
│   └── ui/                 # CLI output utilities
├── .github/workflows/      # CI/CD pipelines
├── Makefile                # Build automation
└── config.json.example     # Example configuration
```

**Key packages:**

- `internal/agent`: Core agent system—daemon lifecycle, task execution, message passing, file-based storage
- `internal/mcp`: MCP server exposing 29 tools (10 config + 19 agent) over stdio
- `internal/tui`: Bubbletea-based terminal UI for visual management
- `internal/commands`: Cobra command tree and CLI implementations

### Building

```bash
make build      # Build binary
make test       # Run tests
make clean      # Clean artifacts
```

## CI/CD

GitHub Actions pipelines:

- **CI**: Runs on every push to `main` and pull requests
- **Release**: Triggered by version tags (`v*`)

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Requirements

- **Go**: 1.24 or later (for building from source)
- **npm**: Required for installing/updating Claude CLI
- **Claude CLI**: Automatically installed if not present

## Troubleshooting

### Claude CLI Not Found

```bash
codes update
```

### API Connection Failed

1. Verify `ANTHROPIC_BASE_URL` is correct
2. Check `ANTHROPIC_AUTH_TOKEN` is valid
3. Ensure network connectivity to the API endpoint

The profile will be saved but marked as "inactive" if validation fails.

### Permission Denied (Linux/macOS)

```bash
sudo ./codes init             # system-wide
# or
mkdir -p ~/bin && cp codes ~/bin/  # user directory
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

[MIT License](LICENSE)

## Acknowledgments

This tool is a wrapper around the official [Claude Code CLI](https://www.npmjs.com/package/@anthropic-ai/claude-code) by Anthropic.

---

**Note**: This is an unofficial tool and is not affiliated with or endorsed by Anthropic.
