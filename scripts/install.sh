#!/usr/bin/env bash
#
# pi-go 一键安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/hwj123hwj/pi-go/main/scripts/install.sh | bash
#
# 也可以直接下载运行:
#   bash scripts/install.sh          → 安装到 ~/.pi-go/bin
#   bash scripts/install.sh /usr/local   → 安装到 /usr/local/bin (需要 sudo)
#
set -euo pipefail

# ── 配置 ──
REPO="hwj123hwj/pi-go"
INSTALL_DIR="${1:-$HOME/.pi-go/bin}"
BINARY_NAME="pi-agent"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}ℹ${NC}  $*"; }
ok()    { echo -e "${GREEN}✓${NC}  $*"; }
warn()  { echo -e "${YELLOW}⚠${NC}  $*"; }
error() { echo -e "${RED}✗${NC}  $*"; exit 1; }

echo ""
echo "  ╔══════════════════════════════════════╗"
echo "  ║        π-go Installer                ║"
echo "  ║   Interactive AI Agent (Bubble Tea)  ║"
echo "  ╚══════════════════════════════════════╝"
echo ""

# ── 检测平台 ──
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    linux)  OS="linux"  ;;
    darwin) OS="darwin" ;;
    *)      error "Unsupported OS: $OS (only linux/darwin supported)" ;;
esac

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)             error "Unsupported architecture: $ARCH" ;;
esac

info "Detected: ${OS}/${ARCH}"

# ── 方案1: 优先用 go install (最简单) ──
if command -v go &>/dev/null; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    info "Found Go $GO_VERSION"

    info "Installing pi-agent via 'go install'..."
    if go install "github.com/${REPO}/cmd/pi-agent@latest" 2>/dev/null; then
        GOPATH_BIN=$(go env GOPATH)/bin
        ok "Installed to ${GOPATH_BIN}/${BINARY_NAME}"
        echo ""

        # 检查 PATH
        case ":$PATH:" in
            *":${GOPATH_BIN}:"*) ;;
            *)
                warn "${GOPATH_BIN} is not in your PATH"
                info "Add this line to your ~/.bashrc or ~/.zshrc:"
                echo ""
                echo -e "  ${CYAN}export PATH=\"\$PATH:${GOPATH_BIN}\"${NC}"
                echo ""
                ;;
        esac

        ok "Done! Start chatting:"
        echo ""
        echo -e "  ${GREEN}${BINARY_NAME} --mode chat${NC}"
        echo ""
        exit 0
    fi
    warn "go install failed, falling back to binary download..."
fi

# ── 方案2: 下载预编译二进制 ──
info "Installing to: ${INSTALL_DIR}"
mkdir -p "${INSTALL_DIR}"

# 确定 GitHub release URL
# 尝试获取最新 release tag
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
    | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' || echo "")

if [ -z "$TAG" ]; then
    warn "No GitHub release found. Trying main branch build..."

    # 没有 release → 检查是否有 go，从源码构建
    if ! command -v go &>/dev/null; then
        error "Neither Go nor a prebuilt binary is available.\nPlease install Go 1.22+ from https://go.dev/dl/"
    fi

    info "Building from source..."
    TMPDIR=$(mktemp -d)
    git clone --depth 1 "https://github.com/${REPO}.git" "${TMPDIR}/pi-go"
    cd "${TMPDIR}/pi-go"
    CGO_ENABLED=0 go build -o "${INSTALL_DIR}/${BINARY_NAME}" ./cmd/pi-agent
    cd - >/dev/null
    rm -rf "${TMPDIR}"
else
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${BINARY_NAME}-${OS}-${ARCH}"

    # 如果 amd64，也试 .tar.gz 格式
    TAR_URL="https://github.com/${REPO}/releases/download/${TAG}/${BINARY_NAME}-${OS}-${ARCH}.tar.gz"

    info "Downloading pi-agent ${TAG}..."
    if curl -fsSL "${TAR_URL}" -o "/tmp/${BINARY_NAME}.tar.gz" 2>/dev/null; then
        tar -xzf "/tmp/${BINARY_NAME}.tar.gz" -C "${INSTALL_DIR}"
        rm -f "/tmp/${BINARY_NAME}.tar.gz"
    elif curl -fsSL "${DOWNLOAD_URL}" -o "${INSTALL_DIR}/${BINARY_NAME}" 2>/dev/null; then
        : # 直接下载的二进制
    else
        error "Failed to download. Check your network or the release assets at:\n  https://github.com/${REPO}/releases"
    fi
fi

chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
ok "Installed to ${INSTALL_DIR}/${BINARY_NAME}"

# ── 添加到 PATH ──
case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
        # 自动添加到 shell 配置
        SHELL_RC=""
        if [ -f "$HOME/.zshrc" ]; then
            SHELL_RC="$HOME/.zshrc"
        elif [ -f "$HOME/.bashrc" ]; then
            SHELL_RC="$HOME/.bashrc"
        fi

        if [ -n "$SHELL_RC" ]; then
            if ! grep -q "${INSTALL_DIR}" "$SHELL_RC" 2>/dev/null; then
                echo "export PATH=\"\$PATH:${INSTALL_DIR}\"" >> "$SHELL_RC"
                ok "Added ${INSTALL_DIR} to PATH in ${SHELL_RC}"
                warn "Run 'source ${SHELL_RC}' or restart your terminal"
            fi
        else
            warn "${INSTALL_DIR} is not in your PATH"
            info "Add this line to your shell config:"
            echo ""
            echo -e "  ${CYAN}export PATH=\"\$PATH:${INSTALL_DIR}\"${NC}"
            echo ""
        fi
        ;;
esac

echo ""
ok "🎉 Installation complete!"
echo ""
echo "  Quick start:"
echo -e "  ${GREEN}${BINARY_NAME} --mode chat${NC}      # Interactive TUI"
echo -e "  ${GREEN}${BINARY_NAME} --mode run -p \"hello\"${NC}  # One-shot"
echo ""
