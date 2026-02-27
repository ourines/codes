# Codes + Feishu Bridge 部署指南

通过飞书 Bot 远程操控云端 Claude Code — 无需 SSH 保持连接。

## 架构

```
飞书用户 → 飞书云 ←[WebSocket 长连接]← Bridge (bridge.mjs)
                                            ↓ HTTP POST
                                      codes serve :3456
                                            ↓
                                      POST /assistant
                                            ↓
                                   assistant.Run() + 19 个工具
                                   ├─ run_tasks → agent team → claude CLI
                                   ├─ get_team_status → 查看进度
                                   ├─ remember/recall → 记忆管理
                                   ├─ set_reminder → 定时提醒
                                   └─ stop_agent / cancel_task → 控制
```

**核心概念：**
- `codes serve` 启动一个常驻进程，监听 HTTP :3456
- `/assistant` 端点是一个完整的 AI 助理（Claude Haiku + tool use），能理解自然语言并自动调度 claude code 进程
- Bridge 只做**飞书 ↔ assistant API 的薄桥接**：收飞书消息 → POST /assistant → 回复飞书

## Bridge 改造总结

| 文件 | 行数 | 说明 |
|------|------|------|
| `bridge/bridge.mjs` | 1271 | 从 feishu-moltbot-bridge fork，删除 Gateway/Device Identity (~360行)，新增 `askAssistant()` |
| `bridge/package.json` | — | 改名 `feishu-codes-bridge`，移除 `ws` 依赖 |
| `bridge/.env.example` | — | 配置模板 |
| `bridge/setup-service.mjs` | — | macOS launchd（暂未适配 Ubuntu，用 systemd 替代）|

**关键改动：**
- `askClawdbot()` → `askAssistant()`: 从 Gateway WebSocket 改为 HTTP POST 到 `localhost:3456/assistant`
- Token 加载: 优先 `CODES_HTTP_TOKEN` 环境变量，否则从 `~/.codes/config.json` 的 `httpTokens[0]` 读取
- 启动健康检查: `GET /health`，失败只警告不阻塞

---

## 一键部署（推荐）

在一台全新的 Ubuntu 24.04 服务器上执行：

```bash
# 方式 1: 本地执行
git clone https://github.com/bigbrother666sh/codes.git
cd codes
chmod +x deploy.sh
./deploy.sh

# 方式 2: 如果你已经在 codes 目录中
./deploy.sh
```

脚本会自动安装 Go、Node.js、Claude Code CLI，编译 codes，配置所有服务，创建 systemd 开机自启。

脚本执行过程中需要你输入：
1. **Anthropic API 配置** — API Key 或代理地址 + Token
2. **飞书 App ID + App Secret** — 从飞书开放平台获取
3. **HTTP Token** — 自动生成，也可自定义
4. **工作目录** — Claude Code 的项目存放路径

备注：

1、如果服务器网络环境极端（比如完全无法访问外网），可以在本机交叉编译后直接传二进制上去：

```
# 本机执行
GOOS=linux GOARCH=amd64 go build -o codes-linux ./cmd/codes
scp codes-linux user@server:/usr/local/bin/codes
```

这样服务器上完全不需要 Go 环境，Phase 2 和 Phase 5 都可以跳过。

2、 如果需要为 claude code 启用 mcp server，需要手动编辑 ~/.claude.json 增加 mcpServers 字段

（可以从本机拷贝）

3、如果使用三方中转的 claude api，还是需要单独为 claude code 配置下转发：

for example:

```
echo 'export ANTHROPIC_BASE_URL="https://ai.ourines.com/api"' >> ~/.bashrc
echo 'export ANTHROPIC_AUTH_TOKEN="你的API密钥"' >> ~/.bashrc
source ~/.bashrc
```

---

## 手动部署（逐步）

如果你更喜欢手动操作，以下是完整步骤。

### 前置准备

你需要准备：
- 一台 Ubuntu 24.04 服务器（推荐 2C4G+）
- [飞书开放平台](https://open.feishu.cn/) 创建一个自建应用，获得 App ID 和 App Secret
- Anthropic API Key（或兼容的第三方代理地址 + Token）

### Step 1: 安装系统依赖

```bash
sudo apt-get update
sudo apt-get install -y git curl wget build-essential jq
```

### Step 2: 安装 Go 1.24+

```bash
# 下载（根据你的架构选择 amd64 或 arm64）
wget https://go.dev/dl/go1.24.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.2.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.profile
source ~/.profile

# 验证
go version  # 应显示 go1.24.2
```

### Step 3: 安装 Node.js 22.x

```bash
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt-get install -y nodejs

# 验证
node --version  # 应显示 v22.x.x
```

### Step 4: 安装 Claude Code CLI

```bash
sudo npm install -g @anthropic-ai/claude-code

# 验证
claude --version
```

### Step 5: 克隆并编译 codes

```bash
git clone https://github.com/bigbrother666sh/codes.git ~/codes
cd ~/codes
go build -o codes ./cmd/codes
sudo cp codes /usr/local/bin/codes

# 验证
codes version
```

### Step 6: 安装 bridge 依赖

```bash
cd ~/codes/bridge
npm install --production
```

### Step 7: 配置 codes

```bash
mkdir -p ~/.codes ~/.codes/secrets ~/.codes/logs ~/.codes/media
```

创建 `~/.codes/config.json`：

```json
{
  "profiles": [
    {
      "name": "default",
      "env": {
        "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
        "ANTHROPIC_AUTH_TOKEN": "sk-ant-你的API-Key"
      },
      "status": "active"
    }
  ],
  "default": "default",
  "skipPermissions": true,
  "defaultBehavior": "current",
  "httpTokens": ["用openssl-rand-hex-32生成"],
  "httpBind": "127.0.0.1:3456",
  "projects_dir": "/home/你的用户名/projects"
}
```

```bash
chmod 600 ~/.codes/config.json
```

> **生成随机 token:** `openssl rand -hex 32`

### Step 8: 配置飞书 secret

```bash
echo -n "你的飞书AppSecret" > ~/.codes/secrets/feishu_app_secret
chmod 600 ~/.codes/secrets/feishu_app_secret
```

### Step 9: 配置 bridge

创建 `~/codes/bridge/.env`：

```bash
FEISHU_APP_ID=cli_你的AppID
FEISHU_APP_SECRET_PATH=/home/你的用户名/.codes/secrets/feishu_app_secret
CODES_HTTP_PORT=3456
CODES_HTTP_TOKEN=与config.json中httpTokens相同的值
```

```bash
chmod 600 ~/codes/bridge/.env
```

### Step 10: 同步 Claude Code 配置

推荐直接从本地机器同步你现有的 Claude Code 配置（settings、rules、plugins 等），
这样云端 claude code 和你本地使用完全一致的配置：

```bash
# 在本地机器执行 (替换 user@server 为服务器地址):
rsync -avz --exclude='.credentials.json' \
  --exclude='history.jsonl' --exclude='sessions/' \
  --exclude='session-env/' --exclude='telemetry/' \
  --exclude='debug/' --exclude='todos/' \
  --exclude='plans/' --exclude='projects/' \
  --exclude='.DS_Store' --exclude='paste-cache/' \
  --exclude='file-history/' --exclude='backups/' \
  ~/.claude/ user@server:~/.claude/
```

排除项说明：
- `.credentials.json` — 本机 OAuth 凭证，服务器需要重新登录
- `sessions/` `session-env/` `history.jsonl` — 会话数据，机器相关
- `telemetry/` `debug/` — 本地遥测和调试日志
- `projects/` — 路径相关的项目配置（含本机路径）

会同步的内容：
- `settings.json` — 模型、语言、插件偏好
- `rules/` — 所有代码规范规则
- `plugins/` — 已安装的插件列表和缓存
- `skills/` — 自定义技能

如果没有现有配置，创建最小配置：

```bash
mkdir -p ~/.claude
cat > ~/.claude/settings.json << 'EOF'
{
  "model": "sonnet",
  "language": "简体中文"
}
EOF
```

### Step 11: 创建 systemd 服务

**codes-serve.service:**

```bash
sudo tee /etc/systemd/system/codes-serve.service << 'EOF'
[Unit]
Description=Codes Serve Daemon
After=network.target

[Service]
Type=simple
User=你的用户名
WorkingDirectory=/home/你的用户名/projects
ExecStart=/usr/local/bin/codes serve
Restart=always
RestartSec=5
Environment=HOME=/home/你的用户名
Environment=PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin

[Install]
WantedBy=multi-user.target
EOF
```

**feishu-bridge.service:**

```bash
sudo tee /etc/systemd/system/feishu-bridge.service << 'EOF'
[Unit]
Description=Feishu Codes Bridge
After=network.target codes-serve.service
Wants=codes-serve.service

[Service]
Type=simple
User=你的用户名
WorkingDirectory=/home/你的用户名/codes/bridge
ExecStart=/usr/bin/node /home/你的用户名/codes/bridge/bridge.mjs
Restart=always
RestartSec=5
Environment=HOME=/home/你的用户名
Environment=PATH=/usr/local/bin:/usr/bin:/bin
EnvironmentFile=/home/你的用户名/codes/bridge/.env

[Install]
WantedBy=multi-user.target
EOF
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable codes-serve feishu-bridge
sudo systemctl start codes-serve
sleep 2
sudo systemctl start feishu-bridge
```

### Step 12: 验证

```bash
# 1. 检查服务状态
sudo systemctl status codes-serve
sudo systemctl status feishu-bridge

# 2. 健康检查
curl http://localhost:3456/health

# 3. 测试 assistant API
curl -H "Authorization: Bearer 你的token" \
     -X POST http://localhost:3456/assistant \
     -H "Content-Type: application/json" \
     -d '{"text":"你好","session_id":"test"}'

# 4. 在飞书中发送消息给 Bot，确认收到回复
```

---

## 飞书应用配置

在 [飞书开放平台](https://open.feishu.cn/) 创建自建应用时，需要：

### 权限

在「权限管理」中开通：
- `im:message` — 获取与发送单聊、群组消息
- `im:message:send_as_bot` — 以应用身份发送消息
- `im:resource` — 获取消息中的资源文件
- `im:chat` — 获取群信息

### 事件订阅

在「事件与回调」中：
- 订阅方式选择 **WebSocket 长连接**（不需要公网 IP）
- 添加事件: `im.message.receive_v1` — 接收消息

### Bot 能力

在「应用能力」中启用「机器人」功能。

---

## 常用操作

### 服务管理

```bash
# 查看状态
sudo systemctl status codes-serve feishu-bridge

# 重启
sudo systemctl restart codes-serve feishu-bridge

# 查看日志（实时）
sudo journalctl -u codes-serve -f
sudo journalctl -u feishu-bridge -f

# 停止
sudo systemctl stop feishu-bridge codes-serve
```

### 更新 codes

```bash
cd ~/codes
git pull
go build -o codes ./cmd/codes
sudo cp codes /usr/local/bin/codes
sudo systemctl restart codes-serve feishu-bridge
```

### 更新 bridge

```bash
cd ~/codes/bridge
npm install --production
sudo systemctl restart feishu-bridge
```

### 查看 Claude 用量

```bash
codes stats summary today
codes stats summary week
codes stats model
```

---

## 通过飞书能做什么

Bridge 连接的是 codes 的 assistant API，它是一个具有以下能力的 AI 助理：

| 能力 | 飞书发送示例 | assistant 调用的工具 |
|------|-------------|---------------------|
| 派发编程任务 | "帮我审查 myproject 的代码" | `run_tasks` → agent team → claude CLI |
| 查看任务进度 | "上次代码审查进展如何？" | `get_team_status` |
| 查看所有团队 | "有哪些正在运行的团队？" | `list_teams` |
| 停止任务 | "停止那个审查任务" | `stop_agent` / `cancel_task` |
| 记忆管理 | "记住我喜欢用 Sonnet 模型" | `remember` |
| 回忆信息 | "我之前说过什么偏好？" | `recall` |
| 定时提醒 | "每天早上9点提醒我检查日志" | `set_schedule` |
| 列出项目 | "我有哪些项目？" | `list_projects` |

所有交互都是**自然语言**，assistant 会自动理解意图并调用对应工具。

---

## 故障排查

### codes serve 无法启动

```bash
# 查看详细错误
sudo journalctl -u codes-serve --no-pager -n 50

# 常见原因：
# 1. config.json 格式错误 → 用 jq 验证: jq . ~/.codes/config.json
# 2. API 配置无效 → codes profile test
# 3. 端口被占用 → ss -tlnp | grep 3456
```

### bridge 无法连接 codes

```bash
# 检查 codes serve 是否在运行
curl http://localhost:3456/health

# 检查 token 是否匹配
# bridge/.env 中的 CODES_HTTP_TOKEN 必须与 ~/.codes/config.json 中的 httpTokens[0] 一致
```

### 飞书收不到回复

```bash
# 检查 bridge 日志
sudo journalctl -u feishu-bridge --no-pager -n 50

# 常见原因：
# 1. App Secret 错误 → 重新写入 ~/.codes/secrets/feishu_app_secret
# 2. 权限未开通 → 检查飞书开放平台的权限配置
# 3. 事件订阅未配置 → 确认 WebSocket 长连接 + im.message.receive_v1
```

### assistant 返回错误

```bash
# 手动测试
curl -H "Authorization: Bearer 你的token" \
     -X POST http://localhost:3456/assistant \
     -H "Content-Type: application/json" \
     -d '{"text":"你好","session_id":"debug"}'

# 常见原因：
# 1. API Key 无效或额度用完
# 2. 网络无法访问 Anthropic API → 检查防火墙/代理配置
```

---

## 安全注意事项

1. `~/.codes/config.json` 包含 API Key，已设置 600 权限，勿公开
2. `bridge/.env` 包含 HTTP Token，已设置 600 权限
3. codes serve 默认监听 `127.0.0.1:3456`（仅本地），bridge 通过本地回环访问，外部无法直接连接
4. 防火墙只需开放 SSH：
   ```bash
   sudo ufw allow ssh
   sudo ufw enable
   ```
   不需要开放 80/443/3456 等端口 — bridge 使用**出站** WebSocket 连接飞书云，无需入站端口
4. 所有 `/assistant` 请求都需要 Bearer Token 认证
