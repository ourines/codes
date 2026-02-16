# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
go build ./cmd/codes                    # Build (quick check)
go build -o codes ./cmd/codes           # Build named binary
go vet ./...                            # Lint
go test ./... -v -count=1               # Run all tests
go test ./internal/commands -run TestX   # Run single test
go test ./internal/session -v            # Run package tests
make build && make test                  # Build + smoke tests
```

Version injection at build time:
```bash
go build -ldflags "-X codes/internal/commands.Version=v1.0.0 -X codes/internal/commands.Commit=$(git rev-parse --short HEAD) -X codes/internal/commands.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o codes ./cmd/codes
```

## Architecture

### Entry Point Routing (`cmd/codes/main.go`)

The root command dynamically selects behavior:
- **TTY detected** → launches bubbletea TUI (`internal/tui`)
- **`--json` flag** → structured JSON output via `internal/output`
- **Non-TTY** → CLI fallback (`internal/commands`)
- **Subcommand** → cobra dispatches to appropriate handler

### Package Responsibilities

| Package | Role |
|---------|------|
| `internal/config` | JSON config load/save, API testing, git info extraction, project/remote CRUD |
| `internal/tui` | Multi-tab bubbletea TUI (Projects, Profiles, Remotes, Settings, Stats, Tasks, Workflows) |
| `internal/session` | Terminal session lifecycle: spawn, track via PID files, kill |
| `internal/remote` | SSH/SCP operations, remote codes installation, profile sync |
| `internal/agent` | Agent team management: daemon lifecycle, task execution, message passing, Claude subprocess orchestration |
| `internal/stats` | Cost tracking: JSONL session parsing, token aggregation, caching, time-range filtering |
| `internal/mcp` | MCP server exposing 40 tools over stdio transport (10 config + 22 agent + 4 stats + 4 workflow tools) |
| `internal/commands` | Cobra command definitions (`cobra.go`) + implementations (`commands.go`) |
| `internal/output` | JSON mode wrapper (`output.JSONMode` flag) |
| `internal/ui` | Styled CLI text output helpers |
| `internal/workflow` | Workflow templates: YAML store, agent team orchestration, legacy migration |

### Command Hierarchy

```
codes
├── init [--yes]
├── update / version
├── doctor                   # Run system diagnostics
├── start (alias: s)         # Launch Claude in directory or project alias
├── profile (alias: pf)      # add / select / test / list / remove
├── project (alias: p)       # add [name] [path] / list / remove
├── config (alias: c)        # set/get/reset/list/export/import (keys: default-behavior, skip-permissions, terminal)
├── remote (alias: r)        # add/remove/list/status/install/sync/setup/ssh
├── agent (alias: a)         # Team and task management
│   ├── team                 # create/delete/list/info
│   ├── add <team> <agent>   # Add agent to team
│   ├── remove <team> <agent>
│   ├── start <team> <agent> # Start agent daemon
│   ├── stop <team> <agent>
│   ├── start-all <team>     # Start all agents
│   ├── stop-all <team>
│   ├── task                 # create/list/get/cancel
│   ├── message              # send/list
│   └── status <team>        # Team dashboard
├── stats (alias: st)        # View Claude API usage statistics
│   ├── summary [period]     # Cost summary (today/week/month/all)
│   ├── project [name]       # Cost breakdown by project
│   ├── model                # Cost breakdown by model
│   └── refresh              # Force cache refresh
├── workflow (alias: wf)     # Workflow templates (agent team orchestration)
│   ├── list                 # List all workflows
│   ├── run <name>           # Launch workflow as agent team
│   ├── create <name>        # Create template scaffold
│   └── delete <name>        # Delete workflow
├── serve                    # MCP server mode
└── completion [shell]       # Hidden, still functional
```

### Config Data Flow

Config lives at `~/.codes/config.json` (fallback: `./config.json` in cwd). Key struct fields:

- `Profiles []APIConfig` — each has `Name`, `Env` map, optional `SkipPermissions`, `Status`
- `Projects map[string]ProjectEntry` — local (string path) or remote (object with `path`+`remote`)
- `Remotes []RemoteHost` — SSH host configurations
- `DefaultBehavior` — startup directory: `current`/`last`/`home`
- `Terminal` — emulator preference: `terminal`/`iterm`/`warp` (macOS) or `auto`/`wt`/`powershell`/`pwsh`/`cmd` (Windows)

**Backward compatibility**: `UnmarshalJSON` on `Config`, `APIConfig`, and `ProjectEntry` handles migration from old formats (flat env vars, `"configs"` field name, string-only projects).

### TUI State Machine (`internal/tui`)

Views: `viewProjects` / `viewProfiles` / `viewRemotes` / `viewSettings` / `viewAddForm` / `viewAddProfile` / `viewAddRemote`

Panel focus: `focusLeft` (list) / `focusRight` (detail/sessions)

Async pattern: long operations return `tea.Cmd` closures that produce typed messages (e.g., `gitCloneMsg`, `remoteStatusMsg`, `sessionTickMsg`). The TUI polls sessions every 3s and remote status every 60s.

### Session Management (`internal/session`)

Sessions spawn Claude in separate terminal windows. Each session gets an ID like `projectname#1`.

Platform-specific terminal launching:
- `terminal_darwin.go` — AppleScript for Terminal.app/iTerm2/Warp
- `terminal_linux.go` — `x-terminal-emulator` or `xterm`
- `terminal_windows.go` — PowerShell/cmd stubs

PID tracking via `/tmp/codes-session-<id>.pid`. `RefreshStatus()` polls process liveness.

### MCP Server (`internal/mcp`)

40 tools registered via `mcpsdk.AddTool()` over stdio transport:

**Config tools (10):** `list_projects`, `add_project`, `remove_project`, `list_profiles`, `switch_profile`, `get_project_info`, `list_remotes`, `add_remote`, `remove_remote`, `sync_remote`

**Stats tools (4):** `stats_summary`, `stats_by_project`, `stats_by_model`, `stats_refresh`

**Agent tools (22):** `team_create`, `team_delete`, `team_list`, `team_get`, `team_status`, `team_start_all`, `team_stop_all`, `agent_add`, `agent_remove`, `agent_list`, `agent_start`, `agent_stop`, `task_create`, `task_update`, `task_list`, `task_get`, `message_send`, `message_list`, `message_mark_read`, `test_sampling`, `team_watch`, `team_subscribe`

**Workflow tools (4):** `workflow_list`, `workflow_get`, `workflow_run`, `workflow_create`

### Agent Team System (`internal/agent`)

The agent system enables multi-agent collaboration through teams of autonomous Claude instances that execute tasks and communicate via message passing.

**Architecture:**

```
Team (TeamConfig)
├── Members (TeamMember[])
│   ├── name, role, model, type
│   └── spawns → Agent Daemon (per member)
├── Tasks (Task[])
│   ├── ID, Subject, Description, Status
│   ├── Owner (assigned agent name)
│   ├── BlockedBy (task dependencies)
│   └── SessionID (persistent Claude session)
└── Messages (Message[])
    ├── From/To (agent names)
    ├── Type (chat, task_completed, task_failed, system)
    └── TaskID (for task reports)
```

**Agent Daemon Lifecycle (`daemon.go`):**

Each agent runs as an independent process (`codes agent run <team> <agent>`), polling at 3-second intervals:

1. **Check stop signal**: Read messages for `__stop__` command
2. **Process messages**: Chat messages routed to Claude subprocess, responses sent back to sender
3. **Find and execute tasks**: Auto-claim pending tasks or execute assigned tasks via Claude subprocess

State tracked in `AgentState` with PID, status (`idle`/`running`/`stopping`/`stopped`), and persistent session ID.

**Agent Start (`team.go`):**

`StartAgent(teamName, agentName)` and `StartAllAgents(teamName)` handle subprocess spawning. These are called by MCP handlers and the workflow runner. Agents are spawned via `os/exec` and detached from the parent process.

**Task Execution (`runner.go`):**

Tasks invoke Claude CLI as subprocess with JSON output:
- `claude -p "<prompt>" --output-format json --session-id <id> --model <model>`
- Output parsed into `ClaudeResult` (result, error, session_id, cost, duration)
- Auto-report completion/failure via broadcast messages (`MsgTaskCompleted`/`MsgTaskFailed`)

**File-based Storage Pattern:**

All state persists in `~/.codes/teams/<team>/`:
- `config.json` — Team configuration (members, workdir)
- `tasks/<id>.json` — Individual task files with atomic writes
- `messages/<id>.json` — Individual message files
- `agents/<name>.json` — Agent state (PID, status, current task)

Atomic writes via temp file + rename. File locks prevent race conditions during task claims.

## Key Patterns

- **Config key naming**: CLI uses kebab-case (`default-behavior`, `skip-permissions`), JSON config uses camelCase (`defaultBehavior`, `skipPermissions`). `RunConfigSet`/`RunConfigGet` accept both forms.
- **Permission resolution**: Per-profile `SkipPermissions *bool` overrides global `Config.SkipPermissions bool`. `nil` means "use global".
- **Remote profile sync**: Only copies `Profiles`/`Default`/`SkipPermissions` to remote — not `Projects` or `LastWorkDir`.
- **Session ID sanitization**: `sanitizeID()` replaces non-alphanumeric chars (except `-`) with `_` for safe file paths.
- **Agent atomic writes**: Task/message files written to temp, then renamed for atomicity. Prevents partial reads during updates.
- **Agent daemon polling**: 3-second poll interval balances responsiveness vs CPU usage. Daemons detach from parent process to survive MCP server restarts.
- **Agent task claiming**: Auto-claim uses read-modify-write pattern with error handling for race conditions. Failed claims are silently skipped (another agent won).
- **Agent file locking**: Future enhancement for coordinated task claims across distributed agents (current impl relies on filesystem atomic renames).
- **Stats caching**: Session data cached in `~/.codes/stats.json` with auto-refresh every 5 minutes. Full rescan via `codes stats refresh` or `stats_refresh` MCP tool.
- **Cost calculation**: Token prices hardcoded in `internal/stats/pricing.go`. Uses Claude API 2024 pricing: input/output/cache-create/cache-read tokens.
- **Workflow team naming**: `wf-{name}-{unix_timestamp}` format. Falls back to milliseconds on collision.
- **Workflow blockedBy mapping**: YAML uses 1-based task indices; runner maps these to actual task IDs via `taskIDMap` during creation.
- **Workflow legacy migration**: `loadWorkflowFile()` detects old `steps` key in YAML and auto-converts to single `worker` agent with sequential `blockedBy` tasks.

### Workflow System (`internal/workflow/`)

Workflows are reusable YAML templates that orchestrate agent teams. Running a workflow creates a team, adds agents, creates tasks with dependencies, and starts all agents — non-blocking.

**Types (`types.go`):**

- `Workflow` — name, description, agents (`[]WorkflowAgent`), tasks (`[]WorkflowTask`), builtIn flag
- `WorkflowAgent` — name, role, model
- `WorkflowTask` — subject, assign (agent name), prompt, priority, blockedBy (1-based task indices)
- `WorkflowRunResult` — teamName, agentsStarted, tasksCreated

**Store (`store.go`):**

- YAML files in `~/.codes/workflows/`
- Built-in workflows: `code-review`, `write-tests`, `pre-pr-check`
- Legacy migration: detects old `steps`-based format and auto-converts to `agents` + `tasks`

**Runner (`runner.go`):**

Non-blocking orchestration flow:
1. Validate agent names and blockedBy references
2. `agent.CreateTeam()` with name `wf-{name}-{timestamp}`
3. `agent.AddMember()` for each workflow agent
4. `agent.CreateTask()` for each task (maps 1-based blockedBy to actual task IDs)
5. `agent.StartAllAgents()` to launch daemons
6. Return `WorkflowRunResult` immediately

On failure at any step, cleans up the partially created team.

**MCP Tools:**

- `workflow_list` — List all workflows (built-in + custom)
- `workflow_get` — Get workflow details
- `workflow_run` — Launch workflow as agent team (returns team name for monitoring)
- `workflow_create` — Create new workflow with validation (agent references, blockedBy bounds)

### Stats System (`internal/stats/`)

The stats system tracks Claude API usage costs by parsing session JSONL files.

**Data Flow:**

1. **Session files** (`~/.claude/projects/<path>/<session-id>.jsonl`) — Claude writes JSONL with `assistant` messages containing `usage` fields
2. **Scanner** (`scanner.go`) — Parses JSONL files, extracts tokens and model info, resolves project aliases from `codes` config
3. **Cache** (`cache.go`) — Stores parsed data in `~/.codes/stats.json` with 5-min refresh interval
4. **Aggregator** (`aggregator.go`) — Groups sessions by day/project/model, supports time-range filtering
5. **Pricing** (`pricing.go`) — Converts token counts to USD costs using hardcoded API pricing

**CLI Commands:**

```bash
codes stats summary [period]  # Overview (today/week/month/all)
codes stats project [name]     # Cost by project
codes stats model              # Cost by model
codes stats refresh            # Force cache rebuild
```

**MCP Tools:**

- `stats_summary` — Get cost summary for time period
- `stats_by_project` — Cost breakdown by project
- `stats_by_model` — Cost breakdown by model
- `stats_refresh` — Force rescan and cache rebuild

**Storage:**

- Cache: `~/.codes/stats.json` (auto-created, auto-refreshed)
- Source: `~/.claude/projects/<dir>/<session>.jsonl` (written by Claude CLI)

## E2E Testing Policy

每个 feature/issue 在关闭前必须通过 e2e 验证。流程：
1. 读取 issue 要求，提取验收标准
2. 执行 `/codes-e2e-test` skill 中对应的测试
3. 所有 checklist 通过后，附结果关闭 issue
4. 新功能必须同时更新 skill 中的测试用例

## CI/CD

- **CI** (`ci.yml`): `go vet`, `go test`, build + smoke tests on ubuntu + windows
- **Release** (`release.yml`): Auto-release on main push. Version bump from commit prefixes: `breaking` → major, `feat` → minor, else → patch. Changelog auto-generated from `git log --grep`. Cross-compiles 6 targets (linux/darwin/windows × amd64/arm64).

Commit messages must use conventional prefixes (`feat:`, `fix:`, `refactor:`, `ci:`, etc.) to trigger releases and generate changelogs correctly.
