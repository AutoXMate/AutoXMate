#!/usr/bin/env bash
set -euo pipefail

REPO="AutoXMate/AutoXmate"
BINARY="autoxmate"
INSTALL_DIR="/usr/local/bin"

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}AutoXMate CLI Installer${NC}"
echo ""

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l|arm) ARCH="arm" ;;
    *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

case "$OS" in
    linux|darwin) ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) echo -e "${RED}Unsupported OS: $OS${NC}"; exit 1 ;;
esac

FILENAME="${BINARY}-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
    FILENAME="${FILENAME}.exe"
fi

DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${FILENAME}"

if command -v $BINARY &> /dev/null; then
    echo -e "  ${BINARY} is already installed at $(which $BINARY)"
    echo ""
fi

echo "  Downloading ${BINARY} for ${OS}/${ARCH}..."
if command -v curl &> /dev/null; then
    curl -fsSL "$DOWNLOAD_URL" -o "/tmp/${FILENAME}" || {
        echo -e "  ${RED}Download failed, falling back to Go install...${NC}"
        GO_FALLBACK=true
    }
elif command -v wget &> /dev/null; then
    wget -q "$DOWNLOAD_URL" -O "/tmp/${FILENAME}" || {
        echo -e "  ${RED}Download failed, falling back to Go install...${NC}"
        GO_FALLBACK=true
    }
else
    echo -e "  ${RED}Neither curl nor wget found, falling back to Go install...${NC}"
    GO_FALLBACK=true
fi

if [ "${GO_FALLBACK:-false}" = false ] && [ -f "/tmp/${FILENAME}" ]; then
    chmod +x "/tmp/${FILENAME}"
    if [ "$(id -u)" = "0" ]; then
        mv "/tmp/${FILENAME}" "${INSTALL_DIR}/${BINARY}"
    else
        echo "  Requesting sudo to install to ${INSTALL_DIR}..."
        sudo mv "/tmp/${FILENAME}" "${INSTALL_DIR}/${BINARY}"
    fi
    echo -e "  ${GREEN}✓ Installed to ${INSTALL_DIR}/${BINARY}${NC}"
elif command -v go &> /dev/null; then
    echo "  Building from source..."
    TMP_DIR=$(mktemp -d)
    git clone --depth 1 "https://github.com/${REPO}.git" "$TMP_DIR" 2>/dev/null || {
        echo -e "  ${RED}Failed to clone repository.${NC}"
        exit 1
    }
    cd "$TMP_DIR"
    go build -o "${BINARY}" -ldflags="-s -w" .
    if [ "$(id -u)" = "0" ]; then
        mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        sudo mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi
    rm -rf "$TMP_DIR"
    echo -e "  ${GREEN}✓ Built and installed to ${INSTALL_DIR}/${BINARY}${NC}"
else
    echo -e "${RED}Could not download binary and Go is not installed.${NC}"
    echo "  Install Go: https://go.dev/dl/"
    echo "  Or download manually from: https://github.com/${REPO}/releases"
    exit 1
fi

echo ""
echo -e "${CYAN}Running 'autoxmate sync' to download tool definitions...${NC}"
${INSTALL_DIR}/${BINARY} sync 2>/dev/null || {
    echo "  Run 'autoxmate sync' later when connected to the internet."
}

echo ""
echo -e "${GREEN}AutoXMate CLI is ready! Run '${BINARY} --help' to get started.${NC}"
echo "  Commands:"
echo "    ${BINARY} list     — List all available tools"
echo "    ${BINARY} status   — Check what's installed"
echo "    ${BINARY} install  — Install tools"
echo "    ${BINARY} run      — Execute a tool"
echo "    ${BINARY} tui      — Interactive TUI"
echo "    ${BINARY} mcp      — MCP server for AI"
echo "    ${BINARY} exec     — Run terminal commands"
echo "    ${BINARY} sync     — Sync tool database"
