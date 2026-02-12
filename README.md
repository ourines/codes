# Codes CLI

A powerful CLI tool for managing multiple Claude Code configurations with ease. Includes an interactive TUI, MCP server integration, and multi-session terminal management.

## Features

- **Interactive TUI**: Full terminal UI with Projects, Profiles, and Settings tabs
- **Multi-Profile Management**: Manage multiple API profiles (official Anthropic, proxies, or alternative providers)
- **Session Manager**: Launch Claude in separate terminal windows with multi-instance support per project
- **MCP Server**: Expose project and profile management as MCP tools for Claude Code integration
- **Smart Directory Launch**: Remember last working directory and support project aliases
- **Environment Import**: Auto-detect and import existing Claude configurations from environment variables
- **API Validation**: Test API connectivity before saving and provide testing tools
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

**Available MCP tools:**

| Tool | Description |
|------|-------------|
| `list_projects` | List all project aliases with git status |
| `add_project` | Add a new project alias |
| `remove_project` | Remove a project alias |
| `list_profiles` | List all API profiles with status |
| `switch_profile` | Switch the default API profile |
| `get_project_info` | Get detailed project info (git branch, dirty status) |

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

## Development

### Project Structure

```
codes/
├── cmd/codes/              # Main entry point
├── internal/
│   ├── commands/           # CLI command implementations
│   ├── config/             # Configuration management
│   ├── mcp/                # MCP server (stdio transport)
│   ├── output/             # JSON output mode
│   ├── session/            # Terminal session manager
│   ├── tui/                # Interactive TUI (bubbletea)
│   └── ui/                 # CLI output utilities
├── .github/workflows/      # CI/CD pipelines
├── Makefile                # Build automation
└── config.json.example     # Example configuration
```

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
