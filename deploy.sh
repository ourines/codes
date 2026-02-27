#!/usr/bin/env bash
# ============================================================================
# Codes + Feishu Bridge — Ubuntu 24.04 一键部署脚本
#
# 用法:
#   curl -fsSL <url>/deploy.sh | bash
#   或: chmod +x deploy.sh && ./deploy.sh
#
# 前置要求: 一台全新的 Ubuntu 24.04 服务器 + root/sudo 权限
# ============================================================================

set -euo pipefail

# ─── 颜色输出 ─────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERROR]${NC} $*"; }
fatal() { err "$*"; exit 1; }

# ─── 配置变量 ─────────────────────────────────────────────────────
GO_VERSION="1.24.2"
NODE_MAJOR=22
CODES_REPO="https://github.com/bigbrother666sh/codes.git"
CODES_DIR="$HOME/codes"
CODES_HTTP_PORT=3456

# ─── 检测环境 ─────────────────────────────────────────────────────
echo ""
echo "============================================"
echo "  Codes + Feishu Bridge 一键部署"
echo "  目标: Ubuntu 24.04"
echo "============================================"
echo ""

if [ "$(id -u)" = "0" ]; then
  warn "检测到 root 用户运行。建议使用普通用户 + sudo。"
  warn "脚本将继续，但 Claude Code 建议以非 root 运行。"
  echo ""
fi

# ─── Phase 1: 系统依赖 ────────────────────────────────────────────
info "Phase 1/7: 安装系统依赖..."

sudo apt-get update -qq
sudo apt-get install -y -qq git curl wget build-essential jq unzip > /dev/null 2>&1
ok "系统依赖已安装"

# ─── Phase 2: Go ──────────────────────────────────────────────────
info "Phase 2/7: 安装 Go ${GO_VERSION}..."

if command -v go &>/dev/null; then
  CURRENT_GO=$(go version | awk '{print $3}' | sed 's/go//')
  ok "Go 已安装: ${CURRENT_GO}"
else
  ARCH=$(dpkg --print-architecture)
  GO_TAR="go${GO_VERSION}.linux-${ARCH}.tar.gz"

  # 优先使用国内镜像下载 Go，失败则回退官方源
  wget -q "https://golang.google.cn/dl/${GO_TAR}" -O "/tmp/${GO_TAR}" 2>/dev/null \
    || wget -q "https://go.dev/dl/${GO_TAR}" -O "/tmp/${GO_TAR}"
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "/tmp/${GO_TAR}"
  rm -f "/tmp/${GO_TAR}"

  # 写入 profile（幂等）
  if ! grep -q '/usr/local/go/bin' "$HOME/.profile" 2>/dev/null; then
    echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> "$HOME/.profile"
  fi
  if ! grep -q 'GOPROXY' "$HOME/.profile" 2>/dev/null; then
    echo 'export GOPROXY=https://goproxy.cn,https://goproxy.io,direct' >> "$HOME/.profile"
  fi
  export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin

  ok "Go $(go version | awk '{print $3}') 已安装"
fi

# ─── Phase 3: Node.js ────────────────────────────────────────────
info "Phase 3/7: 安装 Node.js ${NODE_MAJOR}.x..."

if command -v node &>/dev/null; then
  ok "Node.js 已安装: $(node --version)"
else
  # NodeSource 官方安装
  curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | sudo -E bash - > /dev/null 2>&1
  sudo apt-get install -y -qq nodejs > /dev/null 2>&1
  ok "Node.js $(node --version) 已安装"
fi

# ─── Phase 4: Claude Code CLI ────────────────────────────────────
info "Phase 4/7: 安装 Claude Code CLI..."

if command -v claude &>/dev/null; then
  ok "Claude Code 已安装: $(claude --version 2>/dev/null || echo 'unknown')"
else
  sudo npm install -g @anthropic-ai/claude-code > /dev/null 2>&1
  ok "Claude Code $(claude --version 2>/dev/null || echo '') 已安装"
fi

# ─── Phase 5: 构建 codes ─────────────────────────────────────────
info "Phase 5/7: 克隆并构建 codes..."

if [ -d "$CODES_DIR" ]; then
  info "codes 目录已存在，执行 git pull..."
  cd "$CODES_DIR" && git pull --ff-only
else
  git clone "$CODES_REPO" "$CODES_DIR"
fi

cd "$CODES_DIR"

# 设置 Go 模块代理（国内服务器必需，否则 github.com 依赖下载极慢/超时）
export GOPROXY=https://goproxy.cn,https://goproxy.io,direct
export GONOSUMDB=*
info "使用 Go 模块代理: $GOPROXY"

go mod download
go build -o codes ./cmd/codes
sudo cp codes /usr/local/bin/codes
ok "codes 已构建并安装到 /usr/local/bin/codes"

# ─── Phase 6: 安装 bridge 依赖 ───────────────────────────────────
info "Phase 6/7: 安装 bridge Node.js 依赖..."

cd "$CODES_DIR/bridge"
npm install --production > /dev/null 2>&1
ok "bridge 依赖已安装"

# ─── Phase 7: 交互式配置 ──────────────────────────────────────────
info "Phase 7/7: 配置..."

echo ""
echo "============================================"
echo "  现在需要你提供几个配置值"
echo "============================================"
echo ""

# --- 7a: Anthropic API ---
echo -e "${BLUE}[1/4] API 配置${NC}"
echo "  你使用的是:"
echo "    1) Anthropic 官方 API (api.anthropic.com)"
echo "    2) 第三方代理 / 自部署 API"
read -rp "  选择 (1/2) [1]: " API_CHOICE
API_CHOICE=${API_CHOICE:-1}

if [ "$API_CHOICE" = "1" ]; then
  ANTHROPIC_BASE_URL="https://api.anthropic.com"
  read -rsp "  请输入 Anthropic API Key: " ANTHROPIC_API_KEY
  echo ""
  ANTHROPIC_AUTH_TOKEN="$ANTHROPIC_API_KEY"
else
  read -rp "  请输入 API Base URL: " ANTHROPIC_BASE_URL
  read -rsp "  请输入 Auth Token: " ANTHROPIC_AUTH_TOKEN
  echo ""
fi

# --- 7b: Feishu ---
echo ""
echo -e "${BLUE}[2/4] 飞书配置${NC}"
read -rp "  飞书 App ID (cli_xxx): " FEISHU_APP_ID
read -rsp "  飞书 App Secret: " FEISHU_APP_SECRET
echo ""

# --- 7c: HTTP Token ---
echo ""
echo -e "${BLUE}[3/4] codes HTTP Token${NC}"
CODES_TOKEN=$(openssl rand -hex 32)
echo "  已自动生成 HTTP Token: ${CODES_TOKEN:0:8}..."
read -rp "  使用此 token? (y/自定义输入) [y]: " TOKEN_CHOICE
if [ "$TOKEN_CHOICE" != "y" ] && [ "$TOKEN_CHOICE" != "" ]; then
  CODES_TOKEN="$TOKEN_CHOICE"
fi

# --- 7d: 项目代码仓目录 ---
echo ""
echo -e "${BLUE}[4/4] 项目代码仓目录${NC}"
echo "  这是你要开发的项目 git 仓库存放的目录"
echo "  例如: ~/projects/myapp, ~/projects/backend"
echo "  (不是 codes 自身的数据目录，那个固定在 ~/.codes/)"
read -rp "  项目存放目录 [${HOME}/projects]: " WORK_DIR
WORK_DIR=${WORK_DIR:-"$HOME/projects"}
mkdir -p "$WORK_DIR"

echo ""
info "正在写入配置文件..."

# ─── 写入 codes config ───────────────────────────────────────────
mkdir -p "$HOME/.codes"
mkdir -p "$HOME/.codes/secrets"
mkdir -p "$HOME/.codes/logs"
mkdir -p "$HOME/.codes/media"

cat > "$HOME/.codes/config.json" << CODESEOF
{
  "profiles": [
    {
      "name": "default",
      "env": {
        "ANTHROPIC_BASE_URL": "${ANTHROPIC_BASE_URL}",
        "ANTHROPIC_AUTH_TOKEN": "${ANTHROPIC_AUTH_TOKEN}"
      },
      "status": "active"
    }
  ],
  "default": "default",
  "skipPermissions": true,
  "defaultBehavior": "current",
  "httpTokens": ["${CODES_TOKEN}"],
  "httpBind": "127.0.0.1:${CODES_HTTP_PORT}",
  "projects_dir": "${WORK_DIR}"
}
CODESEOF
chmod 600 "$HOME/.codes/config.json"
ok "codes config 已写入: ~/.codes/config.json"

# ─── 写入飞书 secret ─────────────────────────────────────────────
echo -n "$FEISHU_APP_SECRET" > "$HOME/.codes/secrets/feishu_app_secret"
chmod 600 "$HOME/.codes/secrets/feishu_app_secret"
ok "飞书 secret 已写入: ~/.codes/secrets/feishu_app_secret"

# ─── 写入 bridge .env ────────────────────────────────────────────
cat > "$CODES_DIR/bridge/.env" << BRIDGEEOF
FEISHU_APP_ID=${FEISHU_APP_ID}
FEISHU_APP_SECRET_PATH=${HOME}/.codes/secrets/feishu_app_secret
CODES_HTTP_PORT=${CODES_HTTP_PORT}
CODES_HTTP_TOKEN=${CODES_TOKEN}
BRIDGEEOF
chmod 600 "$CODES_DIR/bridge/.env"
ok "bridge .env 已写入"

# ─── Claude Code 配置 ─────────────────────────────────────────────
echo ""
info "Claude Code 配置..."
if [ -d "$HOME/.claude" ] && [ -f "$HOME/.claude/settings.json" ]; then
  ok "检测到已有 ~/.claude 配置，跳过"
else
  warn "未检测到 ~/.claude 配置"
  echo ""
  echo "  请从本地机器同步 Claude Code 配置到服务器:"
  echo ""
  echo "  # 在本地机器执行 (替换 user@server):"
  echo "  rsync -avz --exclude='.credentials.json' \\"
  echo "    --exclude='history.jsonl' --exclude='sessions/' \\"
  echo "    --exclude='session-env/' --exclude='telemetry/' \\"
  echo "    --exclude='debug/' --exclude='todos/' \\"
  echo "    --exclude='plans/' --exclude='projects/' \\"
  echo "    --exclude='.DS_Store' --exclude='paste-cache/' \\"
  echo "    --exclude='file-history/' --exclude='backups/' \\"
  echo "    ~/.claude/ user@server:~/.claude/"
  echo ""
  echo "  如果你没有现有配置，脚本会创建最小配置..."
  mkdir -p "$HOME/.claude"
  cat > "$HOME/.claude/settings.json" << 'SETTINGSEOF'
{
  "model": "sonnet",
  "language": "简体中文"
}
SETTINGSEOF
  ok "已创建最小 Claude Code 配置"
fi

# ─── 创建 systemd 服务 ───────────────────────────────────────────
info "创建 systemd 服务..."

# codes-serve.service
sudo tee /etc/systemd/system/codes-serve.service > /dev/null << SVCEOF
[Unit]
Description=Codes Serve Daemon (HTTP + MCP + Assistant)
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=${WORK_DIR}
ExecStart=/usr/local/bin/codes serve
Restart=always
RestartSec=5
Environment=HOME=${HOME}
Environment=PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin

[Install]
WantedBy=multi-user.target
SVCEOF

# feishu-bridge.service
sudo tee /etc/systemd/system/feishu-bridge.service > /dev/null << SVCEOF
[Unit]
Description=Feishu Codes Bridge
After=network.target codes-serve.service
Wants=codes-serve.service

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=${CODES_DIR}/bridge
ExecStart=$(which node) ${CODES_DIR}/bridge/bridge.mjs
Restart=always
RestartSec=5
Environment=HOME=${HOME}
Environment=PATH=/usr/local/bin:/usr/bin:/bin
EnvironmentFile=${CODES_DIR}/bridge/.env

[Install]
WantedBy=multi-user.target
SVCEOF

sudo systemctl daemon-reload
sudo systemctl enable codes-serve feishu-bridge
ok "systemd 服务已创建并启用"

# ─── 启动服务 ─────────────────────────────────────────────────────
echo ""
info "启动服务..."

sudo systemctl start codes-serve
sleep 2

# 健康检查
if curl -sf "http://127.0.0.1:${CODES_HTTP_PORT}/health" > /dev/null 2>&1; then
  ok "codes serve 已启动 (端口 ${CODES_HTTP_PORT})"
else
  warn "codes serve 启动中，等待 5 秒..."
  sleep 5
  if curl -sf "http://127.0.0.1:${CODES_HTTP_PORT}/health" > /dev/null 2>&1; then
    ok "codes serve 已启动 (端口 ${CODES_HTTP_PORT})"
  else
    err "codes serve 可能未正常启动，请检查: sudo journalctl -u codes-serve -f"
  fi
fi

sudo systemctl start feishu-bridge
sleep 2

if systemctl is-active --quiet feishu-bridge; then
  ok "feishu-bridge 已启动"
else
  warn "feishu-bridge 可能未正常启动，请检查: sudo journalctl -u feishu-bridge -f"
fi

# ─── 完成 ─────────────────────────────────────────────────────────
echo ""
echo "============================================"
echo -e "  ${GREEN}部署完成!${NC}"
echo "============================================"
echo ""
echo "  服务状态:"
echo "    sudo systemctl status codes-serve"
echo "    sudo systemctl status feishu-bridge"
echo ""
echo "  查看日志:"
echo "    sudo journalctl -u codes-serve -f"
echo "    sudo journalctl -u feishu-bridge -f"
echo ""
echo "  手动测试 assistant API:"
echo "    curl -H 'Authorization: Bearer ${CODES_TOKEN:0:8}...' \\"
echo "         -X POST http://localhost:${CODES_HTTP_PORT}/assistant \\"
echo "         -H 'Content-Type: application/json' \\"
echo "         -d '{\"text\":\"你好\",\"session_id\":\"test\"}'"
echo ""
echo "  重要文件:"
echo "    codes 配置:    ~/.codes/config.json"
echo "    bridge 配置:   ${CODES_DIR}/bridge/.env"
echo "    飞书 secret:   ~/.codes/secrets/feishu_app_secret"
echo "    Claude Code:   ~/.claude/ (从本地 rsync 同步)"
echo ""
echo "  防火墙 (仅需开放 SSH):"
echo "    sudo ufw allow ssh && sudo ufw enable"
echo "    codes serve 仅监听 127.0.0.1，无需额外端口"
echo "    bridge 使用出站 WebSocket 连接飞书，无需入站端口"
echo ""
echo "  现在去飞书发条消息试试吧!"
echo ""
