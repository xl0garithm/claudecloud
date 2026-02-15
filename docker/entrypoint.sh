#!/bin/bash
set -e

# Start a detached Zellij session with the Claude layout
if [ -f /home/claude/.config/zellij/layouts/claude.kdl ]; then
    zellij --session claude --layout claude &
else
    zellij --session claude &
fi

# Keep the container running
exec tail -f /dev/null
