#!/bin/bash
# Startup script for the Zellij shell pane.
# If Claude Code credentials don't exist, launches claude for interactive auth.
# Otherwise, drops to an interactive shell.

CREDS="/home/claude/.claude/.credentials.json"

if [ ! -f "$CREDS" ]; then
    echo ""
    echo "  No Claude Code credentials found."
    echo "  Starting Claude Code for authentication..."
    echo "  Accept the trust prompt, then type /login to sign in."
    echo ""
    claude --dangerously-skip-permissions
    echo ""
    echo "  Authentication complete. You can close this terminal."
    echo ""
fi

exec bash
