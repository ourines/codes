---
name: codes-e2e-test
description: codes CLI 端到端测试套件。覆盖 Project/Profile/Config/Remote/Agent/Workflow/MCP/CLI 全模块 CRUD 生命周期验证，支持按模块或 issue 粒度运行。
---

# codes E2E Test Suite

codes CLI 全功能端到端验证。每个模块遵循 Setup → Execute → Assert → Cleanup 模式。

## Test Modes

| Mode | Purpose |
|------|---------|
| `--all` | Run all modules (1-8) |
| `--module <name>` | Run single module: project, profile, config, remote, agent, workflow, mcp, cli |
| `--issue <number>` | Run issue-specific verification (Module 9) |
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

<step>
**2.1 List existing profiles (baseline):**

```bash
$CODES profile list
```

Assert: Command succeeds. Note existing profile count.
</step>

<step>
**2.2 Add a test profile:**

```bash
$CODES profile add e2e-test-profile <<EOF
ANTHROPIC_API_KEY=sk-test-e2e-fake-key-12345
EOF
```

Assert: Output contains success message.

**Note:** Uses a clearly fake key. This tests the add flow without needing a real API key.
</step>

<step>
**2.3 List profiles — Verify addition:**

```bash
$CODES profile list
```

Assert: `e2e-test-profile` appears in the list.
</step>

<step>
**2.4 Select the test profile:**

```bash
$CODES profile select e2e-test-profile
```

Assert: Exit code 0. Profile is now active.
</step>

<step>
**2.5 Remove test profile:**

```bash
$CODES profile remove e2e-test-profile
```

Assert: Exit code 0. `$CODES profile list` no longer shows `e2e-test-profile`.
</step>

<step>
**2.6 Restore original default profile (if changed):**

If the original default was changed, select it back. Check `$CODES profile list` to confirm.
</step>

---

# Module 3: Config Set/Get/Reset + Export/Import

<step>
**3.1 Get current default-behavior (baseline):**

```bash
ORIGINAL_BEHAVIOR=$($CODES config get default-behavior 2>&1)
echo "Original: $ORIGINAL_BEHAVIOR"
```
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
$CODES config get default-behavior
```

Assert: Output is `home`.
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
$CODES remote add e2e-test-host --host 192.0.2.1 --user testuser --port 2222
```

Assert: Exit code 0. Output confirms addition.

**Note:** Uses TEST-NET IP (RFC 5737) — no actual connection attempted.
</step>

<step>
**4.2 List remotes:**

```bash
$CODES remote list
```

Assert: `e2e-test-host` appears with host `192.0.2.1`, user `testuser`, port `2222`.
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
$CODES agent task create e2e-test-team --subject "E2E test task" --description "Automated test" --assign e2e-worker
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
$CODES agent message send e2e-test-team --from e2e-worker --content "E2E test message"
```

Assert: Exit code 0. Message stored.
</step>

<step>
**5.7 List messages:**

```bash
$CODES agent message list e2e-test-team e2e-worker
```

Assert: Shows the sent message.
</step>

<step>
**5.8 Cleanup — Delete team:**

```bash
$CODES agent team delete e2e-test-team --force
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
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"e2e-test","version":"1.0"}}}' | timeout 5 $CODES serve 2>/dev/null | head -1
```

Assert: Returns JSON-RPC response with server capabilities.
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

# Module 9: Issue Verification

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
| 2 | Profile CRUD | 5 | | | |
| 3 | Config | 6 | | | |
| 4 | Remote | 3 | | | |
| 5 | Agent Team | 8 | | | |
| 6 | Workflow | 2 | | | |
| 7 | MCP Server | 2 | | | |
| 8 | CLI Basics | 6 | | | |

## Summary
- Total: 38 tests
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

# Quick smoke test (project + config + cli)
/codes-e2e-test --quick

# Issue-specific verification
/codes-e2e-test --issue 15
```
