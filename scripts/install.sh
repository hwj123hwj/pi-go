#!/usr/bin/env bash
#
# pi-go 一键安装脚本
#
# 用法:
#   curl -fsSL https://raw.githubusercontent.com/hwj123hwj/pi-go/main/scripts/install.sh | bash
#
# 脚本会自动完成:
#   1. 检测系统 → 下载预编译二进制（或编译源码）
#   2. 安装到 ~/.pi-go/bin 并配置 PATH
#   3. 创建 ~/.pi-go/.env 配置文件
#   4. 引导用户填入 API Key
#   5. 创建 pi-agent 全局命令别名
#
set -euo pipefail

REPO="hwj123hwj/pi-go"
INSTALL_ROOT="${PI_GO_HOME:-$HOME/.pi-go}"
INSTALL_DIR="${INSTALL_ROOT}/bin"
BINARY_NAME="pi-agent"
CONFIG_FILE="${INSTALL_ROOT}/.env"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

info()  { echo -e "${CYAN}ℹ${NC}  $*"; }
ok()    { echo -e "${GREEN}✓${NC}  $*"; }
warn()  { echo -e "${YELLOW}⚠${NC}  $*"; }
fail()  { echo -e "${RED}✗${NC}  $*"; exit 1; }

# ── Banner ──
echo ""
echo -e "${BOLD}  ╔══════════════════════════════════════╗${NC}"
echo -e "${BOLD}  ║        π-go Installer                ║${NC}"
echo -e "${BOLD}  ║   Interactive AI Agent (Bubble Tea)  ║${NC}"
echo -e "${BOLD}  ╚══════════════════════════════════════╝${NC}"
echo ""

# ── 1. 检测平台 ──
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    linux)  OS="linux"  ;;
    darwin) OS="darwin" ;;
    *)      fail "Unsupported OS: $OS (only linux/macOS supported)" ;;
esac
case "$ARCH" in
    x86_64|amd64)  ARCH="amd64"  ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)             fail "Unsupported architecture: $ARCH" ;;
esac
info "Platform: ${OS}/${ARCH}"

# ── 2. 安装二进制 ──
mkdir -p "${INSTALL_DIR}"

BINARY_PATH="${INSTALL_DIR}/${BINARY_NAME}"
NEEDS_DOWNLOAD=true

# 如果已有相同版本，跳过
if [ -f "${BINARY_PATH}" ]; then
    CURRENT_VER=$("${BINARY_PATH}" --version 2>/dev/null | awk '{print $2}' || echo "")
    if [ -n "$CURRENT_VER" ]; then
        info "Found existing pi-agent ${CURRENT_VER}"
        read -rp "$(echo -e ${CYAN}ℹ${NC}  Reinstall/upgrade? [Y/n] )" REPLY < /dev/tty 2>/dev/null || REPLY="y"
        if [[ "${REPLY,,}" == "n" ]]; then
            ok "Keeping existing installation"
            NEEDS_DOWNLOAD=false
        fi
    fi
fi

if [ "$NEEDS_DOWNLOAD" = true ]; then
    # 获取最新 release tag
    info "Fetching latest release..."
    TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
        | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' || echo "")

    DOWNLOADED=false

    if [ -n "$TAG" ]; then
        # 尝试下载预编译二进制
        URL="https://github.com/${REPO}/releases/download/${TAG}/${BINARY_NAME}-${OS}-${ARCH}"
        info "Downloading pi-agent ${TAG}..."
        if curl -fSL "${URL}" -o "${BINARY_PATH}.tmp" 2>/dev/null; then
            mv "${BINARY_PATH}.tmp" "${BINARY_PATH}"
            chmod +x "${BINARY_PATH}"
            DOWNLOADED=true
            ok "Downloaded ${TAG}"
        fi
    fi

    if [ "$DOWNLOADED" = false ]; then
        # 回退：从源码编译
        if command -v go &>/dev/null; then
            info "Building from source (Go detected)..."
            TMPDIR=$(mktemp -d)
            git clone --depth 1 "https://github.com/${REPO}.git" "${TMPDIR}/pi-go" 2>/dev/null
            cd "${TMPDIR}/pi-go"
            CGO_ENABLED=0 go build -ldflags "-X main.version=dev" \
                -o "${BINARY_PATH}" ./cmd/pi-agent
            cd - >/dev/null
            rm -rf "${TMPDIR}"
            ok "Built from source"
        else
            fail "Cannot download binary and Go is not installed.\nPlease install Go from https://go.dev/dl/ and re-run this script."
        fi
    fi
fi

chmod +x "${BINARY_PATH}" 2>/dev/null || true
ok "Binary: ${BINARY_PATH}"

# ── 3. 配置 PATH ──
PATH_CONFIGURED=false
case ":$PATH:" in
    *":${INSTALL_DIR}:"*)
        PATH_CONFIGURED=true
        ;;
esac

if [ "$PATH_CONFIGURED" = false ]; then
    # 检测用户的 shell rc 文件
    SHELL_NAME=$(basename "$SHELL")
    case "$SHELL_NAME" in
        zsh)  RC_FILE="$HOME/.zshrc" ;;
        bash) RC_FILE="$HOME/.bashrc" ;;
        fish) RC_FILE="$HOME/.config/fish/config.fish" ;;
        *)    RC_FILE="$HOME/.profile" ;;
    esac

    # 写入 PATH 配置
    touch "$RC_FILE"
    if ! grep -q "${INSTALL_DIR}" "$RC_FILE" 2>/dev/null; then
        echo "" >> "$RC_FILE"
        echo "# pi-go" >> "$RC_FILE"
        if [ "$SHELL_NAME" = "fish" ]; then
            echo "set -gx PATH \$PATH ${INSTALL_DIR}" >> "$RC_FILE"
        else
            echo "export PATH=\"\$PATH:${INSTALL_DIR}\"" >> "$RC_FILE"
        fi
        ok "PATH configured in ${RC_FILE}"
    fi

    # 当前 session 也生效
    export PATH="$PATH:${INSTALL_DIR}"
fi

# ── 4. 创建快捷封装脚本 ──
# pi-go 是一个 wrapper，支持：
#   pi-go chat          → pi-agent --mode chat
#   pi-go serve         → pi-agent --mode serve
#   pi-go run -p "xxx"  → pi-agent --mode run -p "xxx"
#   pi-go               → pi-agent --mode chat (默认)
WRAPPER="${INSTALL_DIR}/pi-go"
cat > "${WRAPPER}" << 'WRAPEOF'
#!/usr/bin/env bash
# pi-go wrapper — auto-loads config + translates friendly subcommands
export PI_GO_ENV_FILE="${PI_GO_HOME:-$HOME/.pi-go}/.env"

BINARY="${PI_GO_HOME:-$HOME/.pi-go}/bin/pi-agent"

# Subcommand shortcuts
case "${1:-chat}" in
    chat|interactive)
        shift $(($# > 0 ? 1 : 0))
        exec "$BINARY" --mode chat "$@"
        ;;
    serve|server)
        shift
        exec "$BINARY" --mode serve "$@"
        ;;
    run)
        shift
        exec "$BINARY" --mode run "$@"
        ;;
    *)
        # Pass-through: unknown args go straight to pi-agent
        exec "$BINARY" "$@"
        ;;
esac
WRAPEOF
chmod +x "${WRAPPER}"

# ── 5. 配置文件 ──
if [ ! -f "${CONFIG_FILE}" ]; then
    info "Creating config: ${CONFIG_FILE}"

    # 引导用户配置
    echo ""
    echo -e "${BOLD}  ── 配置 API Key ──${NC}"
    echo ""
    echo -e "  选择你的 LLM Provider:"
    echo -e "  ${CYAN}1${NC}) OpenAI / 本地网关 (默认)"
    echo -e "  ${CYAN}2${NC}) Anthropic Claude"
    echo -e "  ${CYAN}3${NC}) 跳过，稍后手动配置"
    echo ""
    read -rp "$(echo -e ${CYAN}ℹ${NC}  选择 [1/2/3]: )" CHOICE < /dev/tty 2>/dev/null || CHOICE="3"

    case "${CHOICE:-1}" in
        1)
            read -rp "$(echo -e ${CYAN}ℹ${NC}  API Key: )" API_KEY < /dev/tty 2>/dev/null || API_KEY=""
            read -rp "$(echo -e ${CYAN}ℹ${NC}  Base URL [http://localhost:4001]: )" BASE_URL < /dev/tty 2>/dev/null || BASE_URL=""
            read -rp "$(echo -e ${CYAN}ℹ${NC}  Model [longcat-opus]: )" MODEL < /dev/tty 2>/dev/null || MODEL=""

            cat > "${CONFIG_FILE}" << EOF
PI_GO_PROVIDER=openai
PI_GO_API_KEY=${API_KEY}
PI_GO_BASE_URL=${BASE_URL:-http://localhost:4001}
PI_GO_MODEL=${MODEL:-longcat-opus}
PI_GO_HOST=127.0.0.1
PI_GO_PORT=8080

# Enable bash tool (allows running shell commands like free, top, ps, etc.)
PI_GO_ENABLE_BASH=true
EOF
            ok "Config saved"
            ;;
        2)
            read -rp "$(echo -e ${CYAN}ℹ${NC}  Anthropic API Key: )" API_KEY < /dev/tty 2>/dev/null || API_KEY=""
            cat > "${CONFIG_FILE}" << EOF
PI_GO_PROVIDER=anthropic
ANTHROPIC_API_KEY=${API_KEY}
ANTHROPIC_MODEL=claude-sonnet-4-20250514
PI_GO_HOST=127.0.0.1
PI_GO_PORT=8080
EOF
            ok "Config saved"
            ;;
        3)
            cat > "${CONFIG_FILE}" << 'EOF'
# pi-go 配置文件
# 请填入你的 API Key 和 Provider 信息

PI_GO_PROVIDER=openai
PI_GO_API_KEY=sk-your-key-here
PI_GO_BASE_URL=http://localhost:4001
PI_GO_MODEL=longcat-opus

# Anthropic (可选)
# PI_GO_PROVIDER=anthropic
# ANTHROPIC_API_KEY=your-key
# ANTHROPIC_MODEL=claude-sonnet-4-20250514
EOF
            warn "Config template created at ${CONFIG_FILE}"
            info "Edit it later: nano ${CONFIG_FILE}"
            ;;
    esac
else
    ok "Config exists: ${CONFIG_FILE}"
fi

# ── 6. 完成 ──
echo ""
echo -e "${GREEN}${BOLD}  🎉 安装完成！${NC}"
echo ""
echo -e "  ${BOLD}下一步：${NC}"
echo ""
if [ "$PATH_CONFIGURED" = false ]; then
    echo -e "  1. 重新加载终端配置:"
    echo -e "     ${CYAN}source ${RC_FILE}${NC}"
    echo ""
fi
echo -e "  ${BOLD}启动交互式 TUI:${NC}"
echo -e "     ${GREEN}pi-go chat${NC}"
echo ""
echo -e "  ${BOLD}单次提问:${NC}"
echo -e "     ${GREEN}pi-go run -p \"你好\"${NC}"
echo ""
echo -e "  ${BOLD}启动 HTTP 服务:${NC}"
echo -e "     ${GREEN}pi-go serve${NC}"
echo ""
echo -e "  安装路径: ${CYAN}${INSTALL_DIR}${NC}"
echo -e "  配置文件: ${CYAN}${CONFIG_FILE}${NC}"
echo ""
