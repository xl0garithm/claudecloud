package handler

import (
	"fmt"
	"net/http"
)

// InstallHandler serves the CLI install script.
type InstallHandler struct {
	baseURL string
}

// NewInstallHandler creates a new InstallHandler.
func NewInstallHandler(baseURL string) *InstallHandler {
	return &InstallHandler{baseURL: baseURL}
}

// ServeScript handles GET /install.sh.
// Returns a shell script that installs the claude-cloud CLI.
func (h *InstallHandler) ServeScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/x-shellscript")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, installScript, h.baseURL, h.baseURL)
}

const installScript = `#!/bin/bash
set -euo pipefail

INSTALL_DIR="$HOME/.local/bin"
mkdir -p "$INSTALL_DIR"

echo "Installing claude-cloud CLI..."

# Download the CLI script
curl -fsSL "%s/cli/claude-cloud" -o "$INSTALL_DIR/claude-cloud"
chmod +x "$INSTALL_DIR/claude-cloud"

# Set the API URL
if ! grep -q "CLAUDE_CLOUD_URL" "$HOME/.bashrc" 2>/dev/null; then
    echo 'export CLAUDE_CLOUD_URL="%s"' >> "$HOME/.bashrc"
fi

echo ""
echo "Installed to $INSTALL_DIR/claude-cloud"
echo ""
echo "Make sure $INSTALL_DIR is in your PATH, then run:"
echo "  claude-cloud login"
`
