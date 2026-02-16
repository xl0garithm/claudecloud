#!/bin/bash
# Idempotent instance setup script.
# Installs mosh, Node.js, Claude Code, Zellij, and creates the claude user.
# Safe to run multiple times — skips already-installed components.
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

echo "=== CloudCode instance setup ==="

# Base packages
if ! command -v mosh &>/dev/null; then
    echo "Installing base packages..."
    apt-get update
    apt-get install -y --no-install-recommends \
        curl git sudo locales ca-certificates mosh ufw
    rm -rf /var/lib/apt/lists/*
else
    echo "Base packages already installed, skipping."
fi

# Locale
locale-gen en_US.UTF-8 2>/dev/null || true
export LANG=en_US.UTF-8

# Node.js 20
if ! command -v node &>/dev/null; then
    echo "Installing Node.js 20..."
    curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
    apt-get install -y nodejs
    rm -rf /var/lib/apt/lists/*
else
    echo "Node.js already installed: $(node --version)"
fi

# Claude Code
if ! command -v claude &>/dev/null; then
    echo "Installing Claude Code..."
    npm install -g @anthropic-ai/claude-code
else
    echo "Claude Code already installed."
fi

# Zellij
if ! command -v zellij &>/dev/null; then
    echo "Installing Zellij..."
    ARCH=$(uname -m)
    case "$ARCH" in
        aarch64|arm64) ZELLIJ_ARCH="aarch64-unknown-linux-musl" ;;
        *)             ZELLIJ_ARCH="x86_64-unknown-linux-musl" ;;
    esac
    curl -fsSL "https://github.com/zellij-org/zellij/releases/latest/download/zellij-${ZELLIJ_ARCH}.tar.gz" \
        | tar -xz -C /usr/local/bin
else
    echo "Zellij already installed: $(zellij --version)"
fi

# Create claude user
if ! id claude &>/dev/null; then
    echo "Creating claude user..."
    useradd -m -s /bin/bash claude
    echo "claude ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
else
    echo "claude user already exists."
fi

# Claude Code config
mkdir -p /home/claude/.config/claude
cat > /home/claude/.config/claude/settings.json <<'SETTINGS'
{"autoApprove": true, "dangerouslyApproveAll": true}
SETTINGS
chown -R claude:claude /home/claude/.config

# ttyd (web terminal)
if ! command -v ttyd &>/dev/null; then
    echo "Installing ttyd..."
    ARCH=$(uname -m)
    case "$ARCH" in
        aarch64|arm64) TTYD_ARCH="aarch64" ;;
        *)             TTYD_ARCH="x86_64" ;;
    esac
    curl -fsSL "https://github.com/tsl0922/ttyd/releases/latest/download/ttyd.${TTYD_ARCH}" \
        -o /usr/local/bin/ttyd
    chmod +x /usr/local/bin/ttyd
else
    echo "ttyd already installed."
fi

# Volume mount point
mkdir -p /claude-data
chown claude:claude /claude-data

# Copy scripts to claude home
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$SCRIPT_DIR/claude-layout.kdl" ]; then
    mkdir -p /home/claude/.config/zellij/layouts
    cp "$SCRIPT_DIR/claude-layout.kdl" /home/claude/.config/zellij/layouts/claude.kdl
    chown -R claude:claude /home/claude/.config/zellij
fi

echo "=== CloudCode instance setup complete ==="
