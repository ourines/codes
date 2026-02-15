# codes Feature Roadmap

> 规划于 2026-02-15，按优先级逐个实现

## 实现顺序

| # | 功能 | 复杂度 | 独特性 | 状态 |
|---|------|--------|--------|------|
| 1 | Session 成本追踪与分析 | 低 | 中 | 🔜 Next |
| 2 | Git Checkpoint / Rollback | 中 | 高 | 📋 Planned |
| 3 | Task 模板 / Workflow Chain | 中 | 高 | 📋 Planned |
| 4 | 跨项目上下文共享 | 中 | 极高 | 📋 Planned |
| 5 | Agent 后台任务队列 + 通知 | 低 | 高 | 📋 Planned |

---

## 1. Session 成本追踪与分析

每次 Claude session 的 cost/token 数据持久化存储，在 TUI 新增 **Stats** tab 展示按项目、按 Profile 的使用统计和趋势图。

### TUI 设计

```
⬡ codes    Projects  Profiles  Remotes  Stats  Settings

╭──────���───── Usage This Week (Feb 10-15) ─────────────────╮
│                                                          │
│  Total: $12.47         Sessions: 23                      │
│  Tokens: 1.2M in / 89K out / 3.4M cached                │
│                                                          │
│  By Project:                                             │
│  ████████████████████░░░░  conduit       $5.21  (42%)    │
│  ████████░░░░░░░░░░░░░░░  copilot-api   $3.14  (25%)    │
│  █████░░░░░░░░░░░░░░░░░░  codes         $2.08  (17%)    │
│  ███░░░░░░��░░░░░░░░░░░░░  others        $2.04  (16%)    │
│                                                          │
│  By Profile:                                             │
│  ████████████████░░░░░░░░  anthropic     $8.33  (67%)    │
│  ████████░░░░░░░░░░░░░░░░  company       $4.14  (33%)    │
│                                                          │
│  Daily Trend:                                            │
│        $3.2 ┤          ╭─╮                               │
│        $2.4 ┤    ╭─╮   │ │                               │
│        $1.6 ┤ ╭──╯ ╰╮  │ ╰──                            │
│        $0.8 ┤─╯     ╰──╯                                │
│             └──────────────────                          │
│              Mon Tue Wed Thu Fri                          │
╰──────────────────────────────────────────────────────────╯

  w: week  m: month  a: all time  p: by project  f: by profile
```

### 数据来源

- Claude `--output-format json` 返回 cost, input_tokens, output_tokens
- 持久化到 `~/.codes/stats.jsonl` (append-only)
- 渲染时按时间范围聚合

---

## 2. Git Checkpoint / Rollback

每次 inline session 前自动创建 git 快照，session 结束后展示回顾页面，支持一键回滚或文件级部分回滚。

### Session Summary 页面

```
╭─────────────────── Session Summary ──────────────────────╮
│                                                          │
│  Project: conduit        Duration: 4m 32s                │
│  Cost: $0.47             Model: opus-4-6                 │
│                                                          │
│  Files Changed (5):                                      │
│    M  src/auth/handler.go         +42  -18               │
│    M  src/auth/middleware.go       +15   -8               │
│    M  src/auth/token.go           +23  -31               │
│    A  src/auth/refresh.go         +67                    │
│    M  internal/config/config.go    +3   -1               │
│                                                          │
│  Net: +150 -58 lines                                     │
╰──────────────────────────────────────────────────────────╯

  d: view diff    r: rollback all    p: partial rollback
  c: commit       enter: keep & return
```

### Partial Rollback 页面

```
╭──────────── Partial Rollback ────────────────╮
│                                              │
│  [✓] src/auth/handler.go        keep         │
│  [✓] src/auth/middleware.go     keep         │
│  [✗] src/auth/token.go         rollback      │
│  [✓] src/auth/refresh.go       keep         │
│  [✗] internal/config/config.go  rollback     │
│                                              │
│  space: toggle   enter: apply   q: cancel    │
╰──────────────────────────────────────────────╯
```

### 技术要点

- **Hook 点**: `tea.ExecProcess` 调用前后
- **Checkpoint**: `git stash create` 或记录 HEAD hash
- **新增视图**: `viewSessionSummary`, `viewSessionDiff`, `viewPartialRollback`
- **涉及文件**: `internal/tui/model.go`, `internal/tui/views.go`, 新增 `internal/session/checkpoint.go`

---

## 3. Task 模板 / Workflow Chain

用户定义可复用的 prompt 模板和多步 workflow，在 TUI 或 CLI 快速执行。

### Workflow 选择

```
╭──────────── Workflows ────────────────────╮
│                                           │
│  Built-in:                                │
│    ▸ Code Review                          │
│    ▸ Write Tests                          │
│    ▸ Pre-PR Check (Review → Test → Docs)  │
│                                           │
│  Custom:                                  │
│    ▸ Security Audit                       │
│    ▸ Refactor to Clean Arch               │
│                                           │
╰───────────────────────────────────────────╯

  enter: run   e: edit   n: new   d: delete
```

### Workflow 定义文件 (`~/.codes/workflows/pre-pr.yml`)

```yaml
name: Pre-PR Check
steps:
  - name: review
    prompt: |
      Review all staged changes. Focus on security, error handling, performance.
    wait_for_approval: true

  - name: test
    prompt: |
      Write tests for all changed files. Follow existing test patterns.
    wait_for_approval: false

  - name: docs
    prompt: |
      Update documentation for any public API changes.
```

### 运行进度

```
╭──────── Running: Pre-PR Check ─────────────╮
│                                            │
│  [✓] Step 1/3: Review          $0.32       │
│      Found 2 issues, 1 suggestion          │
│                                            │
│  [▸] Step 2/3: Write Tests     running...  │
│      ████████████░░░░░░░░                  │
│                                            │
│  [ ] Step 3/3: Update Docs     pending     │
│                                            │
╰────────────────────────────────────────────╯

  space: pause   x: abort   enter: approve & continue
```

---

## 4. 跨项目上下文共享

codes 管理多项目，可在启动 session 时自动注入关联项目的摘要，让 Claude 拥有跨项目全局视角。

### 项目关联配置

```
╭──── Project: copilot-api ────────────────────────╮
│                                                  │
│  Path: ~/Documents/GitHub/copilot-api            │
│  Git: dev ✓ clean                                │
│                                                  │
│  Context Links:                                  │
│    → conduit (API provider)                      │
│      auto-inject: src/routes/*.ts summary        │
│    → noin (deployment target)                    │
│      auto-inject: docker-compose.yml             │
│                                                  │
╰──────────────────────────────────────────────────╯

  l: manage links   enter: open session
```

### 自动注入效果

按 `o` 启动 session 时，codes 在 system prompt 前注入：

```
[Context from linked projects]

## conduit (API provider)
Endpoints: POST /v1/chat/completions, GET /v1/models, POST /v1/sessions/bind
(auto-generated from conduit/src/routes/*.ts)

## noin (deployment)
Services: copilot-api (port 3000), redis, nginx
(auto-generated from docker-compose.yml)
```

---

## 5. Agent 后台任务队列 + 通知

扩展现有 agent 系统为 fire-and-forget 队列模式，支持后台执行和完成通知。

### Task Queue 页面

```
╭──────────── Task Queue ──────────────────────────╮
│                                                  │
│  Queued (3):                                     │
│  #1  conduit      Write unit tests    ⏳ waiting  │
│  #2  copilot-api  Security audit      ⏳ waiting  │
│  #3  noin         Update deps         ⏳ waiting  │
│                                                  │
│  Running (1):                                    │
│  #0  codes        Refactor config     🔄 12m $1.24│
│                                                  │
│  Completed Today (2):                            │
│  ✓  peksy    Add logging      $0.67              │
│  ✓  moduleship  Fix CI        $0.23              │
│                                                  │
╰──────────────────────────────────────────────────╯

  n: new task   enter: view result   x: cancel   p: priority
```

### CLI 接口

```bash
$ codes task add conduit "Write unit tests for auth module"
Task #1 queued for conduit

$ codes task list
#0  codes        Refactor config    running  12m  $1.24
#1  conduit      Write unit tests   waiting
#2  copilot-api  Security audit     waiting

$ codes task result 1
✓ Completed in 8m · $0.89
Files changed: 3 (+245 -12)
```

### 通知

- macOS 原生通知 (完成/失败)
- 可选 webhook 推送 (Slack/飞书/Discord)
