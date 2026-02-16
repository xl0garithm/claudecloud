#!/bin/bash
# Simple process supervisor for Claude instance containers.
# Starts ttyd and the agent, then monitors and restarts on crash.
#
# Zellij is NOT started standalone — ttyd spawns it on-demand via
# "zellij attach claude --create" when a browser connects.
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

# Track backoff per process
TTYD_BACKOFF=1
AGENT_BACKOFF=1

# Start all processes
start_ttyd
start_agent

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
done
