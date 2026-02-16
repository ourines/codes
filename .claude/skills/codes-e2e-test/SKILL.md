---
name: codes-e2e-test
description: codes CLI 端到端测试套件。覆盖 Project/Profile/Config/Remote/Agent/Workflow/MCP/CLI 全模块 CRUD 生命周期验证，支持按模块或 issue 粒度运行。
---

# codes E2E Test Suite

codes CLI 全功能端到端验证。每个模块遵循 Setup → Execute → Assert → Cleanup 模式。

## Test Modes

| Mode | Purpose |
|------|---------|
| `--all` | Run all modules (1-10) |
| `--module <name>` | Run single module: project, profile, config, remote, agent, workflow, mcp, cli, http, notify |
| `--issue <number>` | Run issue-specific verification (Module 11) |
| `--quick` | Modules 1, 3, 8 only (Project + Config + CLI basics) |

## Prerequisites

<step>
**Build the binary before testing:**

```bash
cd /Users/ourines/Projects/codes
go build -o /tmp/codes-e2e-bin ./cmd/codes
```

Verify: `/tmp/codes-e2e-bin version` should output version info.

**Set test binary alias for all subsequent steps:**

```bash
CODES="/tmp/codes-e2e-bin"
```
</step>

---

# Module 1: Project CRUD

<step>
**1.1 Setup — Prepare test directory:**

```bash
TEST_DIR=$(mktemp -d /tmp/codes-e2e-project-XXXXXX)
mkdir -p "$TEST_DIR"
```
</step>

<step>
**1.2 Add project:**

```bash
$CODES project add e2e-test-proj "$TEST_DIR"
```

Assert: Output contains success message. Exit code 0.
</step>

<step>
**1.3 List projects — Verify project appears:**

```bash
$CODES project list
```

Assert: Output contains `e2e-test-proj` and the path `$TEST_DIR`.
</step>

<step>
**1.4 JSON mode — Verify structured output:**

```bash
$CODES project list --json
```

Assert: Valid JSON output. Contains key `e2e-test-proj`.
</step>

<step>
**1.5 Get project info:**

```bash
$CODES --json project list 2>/dev/null | grep -q "e2e-test-proj"
```

Assert: Project info is retrievable.
</step>

<step>
**1.6 Remove project:**

```bash
$CODES project remove e2e-test-proj
```

Assert: Exit code 0. `$CODES project list` no longer shows `e2e-test-proj`.
</step>

<step>
**1.7 Cleanup:**

```bash
rm -rf "$TEST_DIR"
```
</step>

---

# Module 2: Profile CRUD

**Note:** `profile add` and `profile select` are interactive (TTY) commands. This module tests them
via direct config file manipulation as a fallback, or requires manual interaction in a TTY session.

<step>
**2.1 List existing profiles (baseline):**

```bash
$CODES profile list
```

Assert: Command succeeds. Note existing profile count.
</step>

<step>
**2.2 Add a test profile (via config file):**

Since `profile add` is interactive, inject directly into config for automated testing:

```bash
# Read current config, add a test profile, write back
python3 -c "
import json
with open('$HOME/.codes/config.json') as f:
    cfg = json.load(f)
profiles = cfg.get('profiles', [])
profiles.append({'name': 'e2e-test-profile', 'env': {'ANTHROPIC_API_KEY': 'sk-test-fake-key'}})
cfg['profiles'] = profiles
with open('$HOME/.codes/config.json', 'w') as f:
    json.dump(cfg, f, indent=2)
print('Profile injected')
"
```

Assert: `$CODES profile list` shows `e2e-test-profile`.

**For manual testing:** Run `$CODES profile add e2e-test-profile` interactively.
</step>

<step>
**2.3 List profiles — Verify addition:**

```bash
$CODES profile list
```

Assert: `e2e-test-profile` appears in the list.
</step>

<step>
**2.4 Remove test profile:**

```bash
$CODES profile remove e2e-test-profile
```

Assert: Exit code 0. `$CODES profile list` no longer shows `e2e-test-profile`.
</step>

---

# Module 3: Config Set/Get/Reset + Export/Import

<step>
**3.1 Get current default-behavior (baseline):**

```bash
ORIGINAL_BEHAVIOR=$($CODES config get default-behavior 2>&1 | grep -oE '(current|last|home)' | head -1)
echo "Original: $ORIGINAL_BEHAVIOR"
```

Note: `config get` outputs formatted text with description. Use `grep` to extract the actual value.
</step>

<step>
**3.2 Set config value:**

```bash
$CODES config set default-behavior home
```

Assert: Exit code 0.
</step>

<step>
**3.3 Get — Verify change:**

```bash
$CODES config get default-behavior 2>&1 | grep -q "home"
```

Assert: Output contains `home`. Note: `config get` returns formatted text (value + description), not a raw value. Use `grep` to verify.
</step>

<step>
**3.4 Reset config:**

```bash
$CODES config reset default-behavior
```

Assert: Exit code 0. Value returns to default.
</step>

<step>
**3.5 Export config:**

```bash
$CODES config export > /tmp/codes-e2e-config-export.json
```

Assert: File is valid JSON. `cat /tmp/codes-e2e-config-export.json | python3 -m json.tool` succeeds.
Sensitive values should be redacted (contains `[REDACTED]` for any KEY/TOKEN/SECRET/PASSWORD fields).
</step>

<step>
**3.6 Import config roundtrip:**

```bash
$CODES config import /tmp/codes-e2e-config-export.json
```

Assert: Exit code 0. Import succeeds. Redacted values are skipped (not overwritten).
</step>

<step>
**3.7 Cleanup:**

```bash
rm -f /tmp/codes-e2e-config-export.json
# Restore original behavior if needed
$CODES config set default-behavior "$ORIGINAL_BEHAVIOR" 2>/dev/null || true
```
</step>

---

# Module 4: Remote CRUD

<step>
**4.1 Add a test remote (no real SSH needed):**

```bash
$CODES remote add e2e-test-host testuser@192.0.2.1 -p 2222
```

Assert: Exit code 0. Output confirms addition.

**Note:** Uses TEST-NET IP (RFC 5737) — no actual connection attempted.
</step>

<step>
**4.2 List remotes:**

```bash
$CODES remote list
```

Assert: `e2e-test-host` appears with `testuser@192.0.2.1:2222`.
</step>

<step>
**4.3 Remove remote:**

```bash
$CODES remote remove e2e-test-host
```

Assert: Exit code 0. `$CODES remote list` no longer shows `e2e-test-host`.
</step>

---

# Module 5: Agent Team System

<step>
**5.1 Create a test team:**

```bash
$CODES agent team create e2e-test-team
```

Assert: Exit code 0. Team directory created.
</step>

<step>
**5.2 Add an agent to the team:**

```bash
$CODES agent add e2e-test-team e2e-worker --role "test worker" --model haiku
```

Assert: Exit code 0. Agent registered.
</step>

<step>
**5.3 List agents:**

```bash
$CODES agent team info e2e-test-team
```

Assert: Shows `e2e-worker` with role and model info.
</step>

<step>
**5.4 Create a task:**

```bash
$CODES agent task create e2e-test-team "E2E test task" -d "Automated test" --assign e2e-worker
```

Assert: Exit code 0. Task ID returned.
</step>

<step>
**5.5 List tasks:**

```bash
$CODES agent task list e2e-test-team
```

Assert: Shows the created task with status and assignment.
</step>

<step>
**5.6 Send a message:**

```bash
$CODES agent message send e2e-test-team "E2E test message" --from e2e-worker
```

Assert: Exit code 0. Message stored.
</step>

<step>
**5.7 List messages:**

```bash
$CODES agent message list e2e-test-team --agent e2e-worker
```

Assert: Shows the sent message.
</step>

<step>
**5.8 Cleanup — Delete team:**

```bash
$CODES agent team delete e2e-test-team
```

Assert: Exit code 0. Team directory removed.
Verify: `$CODES agent team list` no longer shows `e2e-test-team`.
</step>

---

# Module 6: Workflow

<step>
**6.1 List workflows:**

```bash
$CODES workflow list
```

Assert: Command succeeds. Shows available workflows (may be empty).
</step>

<step>
**6.2 Get workflow details (if any exist):**

If workflows exist from step 6.1, pick the first one:

```bash
$CODES workflow list --json 2>/dev/null
# If a workflow name is found:
# $CODES workflow get <name>
```

Assert: Details returned or graceful "no workflows" message.

**Note:** `workflow run` is NOT tested automatically — it executes real Claude sessions.
</step>

---

# Module 7: MCP Server

<step>
**7.1 Verify MCP server starts (smoke test):**

```bash
# MCP server uses stdio JSON-RPC. Send initialize + newline, capture first response line.
# Note: The server may need both initialize and initialized notification to respond.
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"e2e-test","version":"1.0"}}}\n' | timeout 5 $CODES serve 2>/dev/null | head -1
```

Assert: Returns JSON-RPC response containing `"capabilities"`. If empty, verify via Go tests (7.2) instead —
some MCP implementations require a full handshake (initialize → initialized notification → tool calls) that
single-line piping cannot complete.
</step>

<step>
**7.2 Verify tool registration via Go tests:**

```bash
cd /Users/ourines/Projects/codes && go test ./internal/mcp/... -v -count=1 -run . 2>&1 | tail -20
```

Assert: All MCP tests pass.
</step>

---

# Module 8: CLI Basics

<step>
**8.1 Version:**

```bash
$CODES version
```

Assert: Outputs version string (e.g., `codes version v0.x.x`).
</step>

<step>
**8.2 Doctor:**

```bash
$CODES doctor
```

Assert: Exit code 0. Outputs diagnostic checks.
</step>

<step>
**8.3 Init (non-destructive check):**

```bash
$CODES init --yes 2>&1
```

Assert: Exits cleanly. Config file exists at `~/.codes/config.json`.
</step>

<step>
**8.4 Help output:**

```bash
$CODES --help
```

Assert: Shows usage, available commands, and flags.
</step>

<step>
**8.5 Completion generation:**

```bash
$CODES completion bash 2>/dev/null | head -5
```

Assert: Outputs bash completion script (starts with `#` or `_codes`).
</step>

<step>
**8.6 Unknown command (error handling):**

```bash
$CODES nonexistent-command 2>&1
```

Assert: Non-zero exit code. Error message suggests valid commands.
</step>

---

# Module 9: HTTP Server

**Requires:** `httpTokens` configured in `~/.codes/config.json`. Tests will temporarily inject a test token.

<step>
**9.0 Setup — Configure HTTP server and start it:**

```bash
# Inject test token into config
python3 -c "
import json
with open('$HOME/.codes/config.json') as f:
    cfg = json.load(f)
cfg['httpTokens'] = ['e2e-test-token-12345']
cfg['httpBind'] = ':19876'
with open('$HOME/.codes/config.json', 'w') as f:
    json.dump(cfg, f, indent=2)
print('Config ready')
"

# Add test project for dispatch
$CODES project add e2e-http-proj /tmp 2>&1

# Start HTTP server in background
$CODES serve --http :19876 &
HTTP_PID=$!
sleep 2

# Verify server is running
kill -0 $HTTP_PID 2>/dev/null && echo "Server running (PID=$HTTP_PID)" || echo "FAIL: server not started"
```

Set variables for subsequent steps:
```bash
TOKEN="e2e-test-token-12345"
BASE="http://localhost:19876"
```
</step>

<step>
**9.1 Health check (no auth required):**

```bash
curl -s "$BASE/health" | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['status']=='ok'; print('PASS')"
```

Assert: Returns `{"status":"ok","version":"..."}`. No auth header needed.
</step>

<step>
**9.2 Auth rejection — missing token:**

```bash
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/teams")
```

Assert: HTTP 401. Unauthenticated requests are rejected.
</step>

<step>
**9.3 Auth rejection — wrong token:**

```bash
CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer wrong-token" "$BASE/teams")
```

Assert: HTTP 401. Invalid tokens are rejected (constant-time comparison).
</step>

<step>
**9.4 List teams (authenticated):**

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/teams" | python3 -c "import json,sys; d=json.load(sys.stdin); assert 'teams' in d; print('PASS')"
```

Assert: Returns `{"teams":[...]}` with valid JSON.
</step>

<step>
**9.5 Method not allowed:**

```bash
# POST to health (should be GET only)
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/health")
# GET to dispatch (should be POST only)
CODE2=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN" "$BASE/dispatch")
```

Assert: Both return HTTP 405.
</step>

<step>
**9.6 Content-Type validation:**

```bash
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Authorization: Bearer $TOKEN" -d '{"text":"test"}' "$BASE/dispatch")
```

Assert: HTTP 415 (Unsupported Media Type). POST to `/dispatch` requires `Content-Type: application/json`.
</step>

<step>
**9.7 Dispatch validation — missing text field:**

```bash
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"channel":"test"}' "$BASE/dispatch")
echo "$RESP" | grep -q "text.*required"
```

Assert: HTTP 400. Error message indicates `text` field is required.
</step>

<step>
**9.8 Dispatch — create task via HTTP:**

```bash
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"text":"E2E HTTP test","channel":"e2e","project":"e2e-http-proj","priority":"high"}' \
  "$BASE/dispatch")
```

Assert: HTTP 201. Response contains `task_id`, `team`, `status`. Extract `team` and `task_id` for next steps.

```bash
TEAM=$(echo "$RESP" | python3 -c "import json,sys; print(json.load(sys.stdin)['team'])")
TASK_ID=$(echo "$RESP" | python3 -c "import json,sys; print(json.load(sys.stdin)['task_id'])")
```
</step>

<step>
**9.9 Query dispatched team and task:**

```bash
# Get team details
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/teams/$TEAM" | python3 -c "
import json,sys; d=json.load(sys.stdin)
assert d['name']==sys.argv[1], f'wrong team: {d[\"name\"]}'
assert len(d['members'])>0, 'no members'
print('Team PASS')
" "$TEAM"

# Get task status
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/tasks/$TEAM/$TASK_ID" | python3 -c "
import json,sys; d=json.load(sys.stdin)
assert d['priority']=='high', f'wrong priority: {d[\"priority\"]}'
print(f'Task PASS (status={d[\"status\"]})')
"
```

Assert: Team has members. Task has correct priority.
</step>

<step>
**9.10 Error paths — non-existent resources:**

```bash
# Non-existent team
CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN" "$BASE/teams/nonexistent-xyz")
# Invalid task path
CODE2=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN" "$BASE/tasks/bad")
```

Assert: Team returns 404. Invalid task path returns 400.
</step>

<step>
**9.11 Cleanup:**

```bash
# Delete dispatch team
$CODES agent team delete "$TEAM" 2>/dev/null || true

# Kill HTTP server
kill $HTTP_PID 2>/dev/null || true

# Remove test project
$CODES project remove e2e-http-proj 2>/dev/null || true

# Remove test config
python3 -c "
import json
with open('$HOME/.codes/config.json') as f:
    cfg = json.load(f)
cfg.pop('httpTokens', None)
cfg.pop('httpBind', None)
with open('$HOME/.codes/config.json', 'w') as f:
    json.dump(cfg, f, indent=2)
print('Config cleaned')
"
```
</step>

---

# Module 10: Notification Flow

**Tests the file-based notification system used by agent daemons.**

<step>
**10.1 Setup — Create test team and notification directory:**

```bash
$CODES agent team create e2e-notif-team
mkdir -p ~/.codes/notifications
```
</step>

<step>
**10.2 Write simulated "completed" notification:**

Notifications are normally written by agent daemons (`daemon.writeNotification`).
For e2e testing, simulate by writing the same JSON format:

```bash
cat > ~/.codes/notifications/e2e-notif-team__1.json << 'EOF'
{
  "team": "e2e-notif-team",
  "taskId": 1,
  "subject": "Notification test task",
  "status": "completed",
  "agent": "worker",
  "result": "E2E notification test passed",
  "timestamp": "2026-02-16T15:45:00Z"
}
EOF
```

Assert: File written. Valid JSON. Filename follows `{team}__{taskID}.json` convention.
</step>

<step>
**10.3 Write simulated "failed" notification:**

```bash
cat > ~/.codes/notifications/e2e-notif-team__2.json << 'EOF'
{
  "team": "e2e-notif-team",
  "taskId": 2,
  "subject": "Failing test task",
  "status": "failed",
  "agent": "worker",
  "error": "Simulated failure for e2e testing",
  "timestamp": "2026-02-16T15:45:01Z"
}
EOF
```

Assert: File written. Contains `"error"` field (not `"result"`).
</step>

<step>
**10.4 Verify notification format and content:**

```bash
python3 -c "
import json, os, glob

files = sorted(glob.glob(os.path.expanduser('~/.codes/notifications/e2e-notif-team__*.json')))
assert len(files) == 2, f'Expected 2 files, got {len(files)}'

for f in files:
    with open(f) as fh:
        n = json.load(fh)
    assert n['team'] == 'e2e-notif-team'
    assert n['status'] in ('completed', 'failed')
    assert 'timestamp' in n
    if n['status'] == 'completed':
        assert 'result' in n, 'completed notification missing result'
    else:
        assert 'error' in n, 'failed notification missing error'
    print(f'  {os.path.basename(f)}: status={n[\"status\"]} OK')

print('PASS')
"
```

Assert: Both files parse correctly. Completed has `result`, failed has `error`.
</step>

<step>
**10.5 Verify Go notification tests pass:**

```bash
go test ./internal/notify/... -count=1 -v 2>&1 | tail -10
go test ./internal/mcp/... -count=1 -run "Subscribe|Notif|Monitor" -v 2>&1 | tail -15
```

Assert: All notify and MCP notification tests pass. Key tests:
- `TestE2E_PiggybackNotifications` — notifications delivered via MCP tool responses
- `TestE2E_TeamSubscribeReceivesNotification` — blocking subscribe receives notifications
- `TestE2E_TeamSubscribeFiltersTeam` — team-level isolation works
</step>

<step>
**10.6 Cleanup:**

```bash
rm -f ~/.codes/notifications/e2e-notif-team__*.json
$CODES agent team delete e2e-notif-team 2>/dev/null || true
```
</step>

---

# Module 11: Issue Verification

<step>
**9.1 Read the issue:**

Usage: `/codes-e2e-test --issue <number>`

```bash
gh issue view <number> --json title,body,labels
```

Extract acceptance criteria from the issue body.
</step>

<step>
**9.2 Build checklist from issue:**

Parse the issue body for:
- [ ] items (explicit checklist)
- "should", "must", "verify" keywords (implicit criteria)
- Code examples (commands to test)

Create a verification plan.
</step>

<step>
**9.3 Execute each checklist item:**

For each criterion:
1. Run the command or operation described
2. Verify the expected output/behavior
3. Record PASS/FAIL with actual output

**Example for Issue #15 (config export/import):**

```bash
# Criterion 1: export produces valid JSON
$CODES config export > /tmp/issue-test.json
python3 -m json.tool /tmp/issue-test.json >/dev/null 2>&1 && echo "PASS" || echo "FAIL"

# Criterion 2: sensitive values are redacted
grep -c "REDACTED" /tmp/issue-test.json

# Criterion 3: import roundtrip works
$CODES config import /tmp/issue-test.json && echo "PASS" || echo "FAIL"

# Cleanup
rm -f /tmp/issue-test.json
```
</step>

<step>
**9.4 Generate issue verification report:**

```markdown
## Issue #<N> Verification Report

**Title:** <issue title>
**Date:** <date>
**Binary:** <$CODES version output>

| # | Criterion | Status | Output |
|---|-----------|--------|--------|
| 1 | ... | PASS/FAIL | ... |
| 2 | ... | PASS/FAIL | ... |

**Result:** ALL PASS / X FAILED
```
</step>

---

# Test Report Template

<step>
**Generate final report after all modules:**

```markdown
# codes E2E Test Report

## Environment
- Platform: <os/arch>
- Date: <date>
- Binary: <$CODES version>
- Go: <go version>

## Results

| # | Module | Tests | Passed | Failed | Status |
|---|--------|-------|--------|--------|--------|
| 1 | Project CRUD | 6 | | | |
| 2 | Profile CRUD | 3 | | | |
| 3 | Config | 6 | | | |
| 4 | Remote | 3 | | | |
| 5 | Agent Team | 8 | | | |
| 6 | Workflow | 2 | | | |
| 7 | MCP Server | 2 | | | |
| 8 | CLI Basics | 6 | | | |
| 9 | HTTP Server | 11 | | | |
| 10 | Notification Flow | 5 | | | |

## Summary
- Total: 52 tests
- Passed: X
- Failed: X
- Verdict: PASS / FAIL
```
</step>

---

# Invocation

```bash
# Full test suite
/codes-e2e-test --all

# Single module
/codes-e2e-test --module project
/codes-e2e-test --module config
/codes-e2e-test --module agent
/codes-e2e-test --module http
/codes-e2e-test --module notify

# Quick smoke test (project + config + cli)
/codes-e2e-test --quick

# Issue-specific verification
/codes-e2e-test --issue 15
```
