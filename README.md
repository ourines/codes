# Codes CLI

A powerful CLI tool for managing multiple Claude Code configurations with ease. Switch between different Claude API endpoints, manage authentication tokens, and streamline your AI-powered development workflow.

## Features

- **Multi-Configuration Management**: Manage multiple Claude API configurations (official Anthropic, proxies, or alternative providers)
- **Easy Switching**: Quickly switch between configurations with an interactive selector
- **Smart Directory Launch**: Remember last working directory and support project aliases for quick access
- **Environment Import**: Automatically detect and import existing Claude configurations from environment variables
- **Automatic Installation**: Automatically installs and updates Claude CLI when needed
- **API Validation**: Tests API connectivity before saving configurations
- **Cross-Platform**: Support for Linux, macOS, and Windows (amd64 & arm64)
- **Zero Configuration**: Works out of the box with sensible defaults

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/ourines/codes/releases).

#### Linux / macOS

```bash
# Download and extract (example for linux-amd64)
curl -L https://github.com/ourines/codes/releases/latest/download/codes-linux-amd64 -o codes
chmod +x codes

# Install to system path
./codes install
```

#### Windows

```powershell
# Download the binary for your architecture
# Then run the install command
.\codes.exe install
```

### Build from Source

Requirements:
- Go 1.21 or later
- npm (required for Claude CLI installation)

```bash
# Clone the repository
git clone https://github.com/ourines/codes.git
cd codes

# Build the binary
make build

# Install to system PATH
./codes install
```

## Quick Start

### 1. Check Your Environment (Optional but Recommended)

```bash
codes init
```

This will verify that everything is set up correctly and guide you if something is missing.

### 2. Add Your First Configuration

```bash
codes add
```

You'll be prompted to enter:
- Configuration name (e.g., "official", "proxy")
- `ANTHROPIC_BASE_URL` (e.g., `https://api.anthropic.com`)
- `ANTHROPIC_AUTH_TOKEN` (your API key)

The tool will automatically test the API connection before saving.

### 3. Run Claude with Your Configuration

```bash
codes
```

This will launch Claude CLI with the selected configuration's environment variables.

### 4. Switch Between Configurations

```bash
codes select
```

An interactive menu will appear showing all your configurations. Select one to switch and launch Claude.

## Commands

### `codes init`

Check your environment and validate your configuration. This command performs comprehensive checks including:

- Verifies Claude CLI installation
- Detects existing Claude configuration in environment variables
- Offers to import existing configuration if found
- Checks configuration file existence and validity
- Tests API connectivity for the default configuration
- Displays detailed status of all configurations

```bash
codes init
```

**Example output:**
```
✓ Claude CLI is installed
✓ Found existing configuration in environment variables
✓ Configuration file exists
✓ Found 3 configuration(s)
✓ Default configuration is working
```

This is a great command to run after installation or when troubleshooting issues.

### `codes` (no arguments)

Runs Claude CLI with the currently selected configuration in the last used directory. If Claude CLI is not installed, it will be automatically installed. The tool remembers your last working directory for convenience.

```bash
codes
```

You can also specify a directory or project alias:

```bash
codes /path/to/project
codes my-project  # if you've added a project alias
```

### `codes add`

Interactively add a new Claude API configuration.

```bash
codes add
```

### `codes select`

Display all configurations and interactively select one to use.

```bash
codes select
```

### `codes update`

Update or install a specific version of Claude CLI.

```bash
codes update
```

Lists the latest 20 available versions from npm and allows you to:
- Select a version by number (1-20)
- Enter a specific version number (e.g., `1.2.3`)
- Type `latest` to install the newest version

### `codes install`

Install the codes binary to your system PATH.

```bash
codes install
```

- **Linux/macOS**: Installs to `/usr/local/bin` or `~/bin`
- **Windows**: Installs to `~/go/bin`

### `codes start [path-or-project]`

Start Claude Code in a specific directory or using a project alias. Without arguments, it uses the last working directory.

```bash
# Start in current directory (and remember it)
codes start .

# Start in specific path
codes start /path/to/project

# Start using project alias
codes start my-project
```

### `codes project add <name> <path>`

Add a project alias for quick access to frequently used directories.

```bash
codes project add my-app /path/to/my-app
```

### `codes project list`

List all configured project aliases.

```bash
codes project list
```

### `codes project remove <name>`

Remove a project alias.

```bash
codes project remove my-app
```

### `codes version`

Display the current version of codes CLI.

```bash
codes version
```

## Configuration

### Configuration File Location

The tool searches for `config.json` in the following order:

1. Current working directory: `./config.json`
2. User home directory: `~/.codes/config.json`

### Configuration Format

Create a `config.json` file with the following structure:

```json
{
  "configs": [
    {
      "name": "official",
      "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
      "ANTHROPIC_AUTH_TOKEN": "sk-ant-xxxxx"
    },
    {
      "name": "proxy",
      "ANTHROPIC_BASE_URL": "https://your-proxy.com/api",
      "ANTHROPIC_AUTH_TOKEN": "your-token"
    }
  ],
  "default": "official"
}
```

### Configuration Fields

- `name`: Unique identifier for the configuration
- `ANTHROPIC_BASE_URL`: Base URL for the Claude API endpoint
- `ANTHROPIC_AUTH_TOKEN`: Authentication token for the API
- `status`: (optional) API status - "active", "inactive", or "unknown"
- `default`: The configuration name to use by default

### Example Configurations

See `config.json.example` for a complete example with multiple providers:

```bash
cp config.json.example config.json
# Edit config.json with your actual tokens
```

## Development

### Project Structure

```
codes/
├── cmd/codes/          # Main entry point
├── internal/
│   ├── commands/       # Command implementations
│   ├── config/         # Configuration management
│   └── ui/             # User interface utilities
├── .github/workflows/  # CI/CD pipelines
├── Makefile           # Build automation
└── config.json.example # Example configuration
```

### Building

```bash
# Build the binary
make build

# Run tests
make test

# Clean build artifacts
make clean

# Display version
make version
```

### Running Tests

```bash
make test
```

## CI/CD

This project uses GitHub Actions for continuous integration and automated releases:

- **CI Pipeline**: Runs on every push to `main` and pull requests
- **Release Pipeline**: Triggered by version tags (e.g., `v1.0.0`)

### Creating a Release

```bash
# Tag a new version
git tag v1.0.0
git push origin v1.0.0
```

This will automatically build binaries for all supported platforms and create a GitHub release.

## Requirements

- **Go**: 1.21 or later (for building from source)
- **npm**: Required for installing/updating Claude CLI
- **Claude CLI**: Automatically installed by the tool if not present

## Troubleshooting

### Claude CLI Not Found

If the tool can't find Claude CLI, run:

```bash
codes update
```

This will install the latest version of Claude CLI via npm.

### API Connection Failed

If API validation fails when adding a configuration:

1. Verify your `ANTHROPIC_BASE_URL` is correct
2. Check that your `ANTHROPIC_AUTH_TOKEN` is valid
3. Ensure you have network connectivity to the API endpoint

The configuration will still be saved but marked as "inactive" if validation fails.

### Permission Denied (Linux/macOS)

If you get permission errors during installation:

```bash
# Use sudo for system-wide installation
sudo ./codes install

# Or install to user directory
mkdir -p ~/bin
cp codes ~/bin/
export PATH="$HOME/bin:$PATH"
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

[MIT License](LICENSE) - feel free to use this project for any purpose.

## Acknowledgments

This tool is a wrapper around the official [Claude Code CLI](https://www.npmjs.com/package/@anthropic-ai/claude-code) by Anthropic.

---

**Note**: This is an unofficial tool and is not affiliated with or endorsed by Anthropic.
