# codes doctor - System Diagnostics

The `codes doctor` command runs comprehensive system diagnostics to verify your Claude Code environment is properly configured and healthy.

## Usage

```bash
codes doctor
```

## What It Checks

### 1. Claude CLI Installation
- ✓ Verifies Claude CLI is in PATH
- ✓ Displays Claude CLI version
- ✗ Suggests installation command if missing

### 2. Configuration File
- ✓ Confirms config file exists and is valid
- ✓ Shows number of configured API profiles
- ✓ Displays default profile name
- ! Warns if no profiles configured
- ✗ Fails if config is corrupted

### 3. API Connectivity
- ✓ Tests connection to default API profile
- ! Skips test if no profiles configured
- ✗ Fails if API is unreachable

### 4. File Permissions
- ✓ Checks config directory (`~/.codes/`) is writable
- ✓ Checks session directory (`~/.claude/`) is writable
- ✗ Fails if critical directories aren't writable

### 5. Agent Daemons
- ✓ Lists all teams and agents
- ✓ Shows running/stopped status for each agent
- ℹ Provides command to start stopped agents
- ! Warns if unable to check team status

### 6. Disk Space
- ℹ Shows Claude session data size
- ℹ Shows codes data size
- ℹ Displays disk usage percentage
- ✓ Confirms sufficient space available
- ! Warns if disk usage > 90% or < 1GB free
- ✗ Fails if disk is critically full

## Exit Codes

- `0` - All checks passed (may have warnings)
- `1` - One or more critical checks failed

## Example Output

```
 ────────────────────────────
 Running System Diagnostics
 ────────────────────────────

1. Checking Claude CLI...
 ✓ Claude CLI found: /usr/local/bin/claude
 ℹ Version: 2.1.39 (Claude Code)

2. Checking configuration file...
 ✓ Config loaded: /Users/user/.codes/config.json
 ✓ 3 profile(s) configured
 ✓ Default profile: anthropic

3. Checking API connectivity...
 ℹ Testing profile: anthropic
 ✓ API connection successful

4. Checking file permissions...
 ✓ Config directory writable: /Users/user/.codes
 ✓ Session directory writable: /Users/user/.claude

5. Checking agent daemons...
 ✓ 3/3 agents running across 1 team(s)

6. Checking disk space...
 ℹ Claude session data: /Users/user/.claude (1.7 GB)
 ℹ Codes data: /Users/user/.codes (56.9 KB)
 ℹ Disk usage: 14.4% (792.7 GB / 926.4 GB available)
 ✓ Sufficient disk space available

 ────────────────────
 Diagnostic Summary
 ────────────────────
  ✓ Passed: 8

 ✓ All checks passed!
```

## When to Use

Run `codes doctor` when:
- Setting up codes for the first time
- Troubleshooting connection issues
- Before opening a support ticket
- After system updates
- When agent daemons aren't responding
- If session data seems corrupted

## Troubleshooting

### Claude CLI Not Found
```bash
npm install -g @anthropic-ai/claude
```

### No API Profiles
```bash
codes profile add
```

### Disk Space Low
Clean up old Claude sessions:
```bash
# Review session data
ls -lh ~/.claude/

# Remove old sessions if needed
rm -rf ~/.claude/sessions/old-session-id
```

### Agents Not Running
```bash
# Start all agents in a team
codes agent start-all <team-name>

# Or start specific agent
codes agent start <team-name> <agent-name>
```
