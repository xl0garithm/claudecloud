#!/bin/bash
set -e

# Start ttyd web terminal (proxies to Zellij session)
ttyd -W -p 7681 zellij attach claude --create &

# Start instance agent (chat, files, projects)
cd /opt/cloudcode/agent && node server.js &

# Start a detached Zellij session with the Claude layout
if [ -f /home/claude/.config/zellij/layouts/claude.kdl ]; then
    zellij --session claude --layout claude &
else
    zellij --session claude &
fi

# Keep the container running
exec tail -f /dev/null
