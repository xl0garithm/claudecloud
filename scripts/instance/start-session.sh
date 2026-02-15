#!/bin/bash
# Start or attach to the named Zellij session with the Claude layout.
set -euo pipefail

SESSION_NAME="${1:-claude}"
LAYOUT="claude"

# If session exists, attach to it
if zellij list-sessions 2>/dev/null | grep -q "^${SESSION_NAME}"; then
    exec zellij attach "$SESSION_NAME"
fi

# Otherwise, create a new session with the layout
exec zellij --session "$SESSION_NAME" --layout "$LAYOUT"
