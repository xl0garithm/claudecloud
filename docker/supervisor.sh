#!/bin/bash
# Simple process supervisor for Claude instance containers.
# Starts ttyd, agent, and zellij, then monitors and restarts on crash.
set -u

MAX_BACKOFF=30

start_ttyd() {
    ttyd -W -p 7681 zellij attach claude --create &
    TTYD_PID=$!
    echo "supervisor: started ttyd (pid=$TTYD_PID)" >&2
}

start_agent() {
    cd /opt/cloudcode/agent && node server.js &
    AGENT_PID=$!
    echo "supervisor: started agent (pid=$AGENT_PID)" >&2
}

start_zellij() {
    if [ -f /home/claude/.config/zellij/layouts/claude.kdl ]; then
        zellij --session claude --layout claude &
    else
        zellij --session claude &
    fi
    ZELLIJ_PID=$!
    echo "supervisor: started zellij (pid=$ZELLIJ_PID)" >&2
}

# Track backoff per process
TTYD_BACKOFF=1
AGENT_BACKOFF=1
ZELLIJ_BACKOFF=1

# Start all processes
start_ttyd
start_agent
start_zellij

# Monitor loop
while true; do
    sleep 5

    # Check ttyd
    if ! kill -0 "$TTYD_PID" 2>/dev/null; then
        echo "supervisor: ttyd exited, restarting (backoff=${TTYD_BACKOFF}s)" >&2
        sleep "$TTYD_BACKOFF"
        start_ttyd
        TTYD_BACKOFF=$((TTYD_BACKOFF * 2))
        if [ "$TTYD_BACKOFF" -gt "$MAX_BACKOFF" ]; then
            TTYD_BACKOFF=$MAX_BACKOFF
        fi
    else
        TTYD_BACKOFF=1
    fi

    # Check agent
    if ! kill -0 "$AGENT_PID" 2>/dev/null; then
        echo "supervisor: agent exited, restarting (backoff=${AGENT_BACKOFF}s)" >&2
        sleep "$AGENT_BACKOFF"
        start_agent
        AGENT_BACKOFF=$((AGENT_BACKOFF * 2))
        if [ "$AGENT_BACKOFF" -gt "$MAX_BACKOFF" ]; then
            AGENT_BACKOFF=$MAX_BACKOFF
        fi
    else
        AGENT_BACKOFF=1
    fi

    # Check zellij
    if ! kill -0 "$ZELLIJ_PID" 2>/dev/null; then
        echo "supervisor: zellij exited, restarting (backoff=${ZELLIJ_BACKOFF}s)" >&2
        sleep "$ZELLIJ_BACKOFF"
        start_zellij
        ZELLIJ_BACKOFF=$((ZELLIJ_BACKOFF * 2))
        if [ "$ZELLIJ_BACKOFF" -gt "$MAX_BACKOFF" ]; then
            ZELLIJ_BACKOFF=$MAX_BACKOFF
        fi
    else
        ZELLIJ_BACKOFF=1
    fi
done
