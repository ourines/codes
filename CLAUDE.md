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
| `internal/tui` | Multi-tab bubbletea TUI (Projects, Profiles, Remotes, Settings) |
| `internal/session` | Terminal session lifecycle: spawn, track via PID files, kill |
| `internal/remote` | SSH/SCP operations, remote codes installation, profile sync |
| `internal/mcp` | MCP server exposing 10 tools over stdio transport |
| `internal/commands` | Cobra command definitions (`cobra.go`) + implementations (`commands.go`) |
| `internal/output` | JSON mode wrapper (`output.JSONMode` flag) |
| `internal/ui` | Styled CLI text output helpers |

### Command Hierarchy

```
codes
├── init [--yes]
├── update / version
├── start (alias: s)         # Launch Claude in directory or project alias
├── profile (alias: pf)      # add / select / test / list / remove
├── project (alias: p)       # add [name] [path] / list / remove
├── config (alias: c)        # set/get/reset/list (keys: default-behavior, skip-permissions, terminal)
├── remote (alias: r)        # add/remove/list/status/install/sync/setup/ssh
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

10 tools registered via `mcpsdk.AddTool()`: `list_projects`, `add_project`, `remove_project`, `list_profiles`, `switch_profile`, `get_project_info`, `list_remotes`, `add_remote`, `remove_remote`, `sync_remote`. Runs over stdio transport.

## Key Patterns

- **Config key naming**: CLI uses kebab-case (`default-behavior`, `skip-permissions`), JSON config uses camelCase (`defaultBehavior`, `skipPermissions`). `RunConfigSet`/`RunConfigGet` accept both forms.
- **Permission resolution**: Per-profile `SkipPermissions *bool` overrides global `Config.SkipPermissions bool`. `nil` means "use global".
- **Remote profile sync**: Only copies `Profiles`/`Default`/`SkipPermissions` to remote — not `Projects` or `LastWorkDir`.
- **Session ID sanitization**: `sanitizeID()` replaces non-alphanumeric chars (except `-`) with `_` for safe file paths.

## CI/CD

- **CI** (`ci.yml`): `go vet`, `go test`, build + smoke tests on ubuntu + windows
- **Release** (`release.yml`): Auto-release on main push. Version bump from commit prefixes: `breaking` → major, `feat` → minor, else → patch. Changelog auto-generated from `git log --grep`. Cross-compiles 6 targets (linux/darwin/windows × amd64/arm64).

Commit messages must use conventional prefixes (`feat:`, `fix:`, `refactor:`, `ci:`, etc.) to trigger releases and generate changelogs correctly.
