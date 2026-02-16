#!/bin/bash
# Connect to the persistent Zellij session.
# Called by ttyd for each browser connection. Handles two cases:
#   1. Session exists → attach to it (subsequent connections)
#   2. Session doesn't exist → create it with the Claude layout (first connection)
#
# When the browser disconnects, only the Zellij client exits; the server and
# all running processes (including Claude) continue working in the background.

SESSION="main"
LAYOUT="/home/claude/.config/zellij/layouts/claude.kdl"

# Try to attach to an existing session
if zellij list-sessions 2>/dev/null | grep -q "^${SESSION}"; then
    exec zellij attach "${SESSION}"
fi

# No existing session — create one with the Claude layout.
# `-s` names the session; `--layout` loads the pane arrangement.
exec zellij -s "${SESSION}" --layout "${LAYOUT}"
