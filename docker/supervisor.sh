#!/bin/bash
# Process supervisor for Claude instance containers.
#
# Architecture:
#   Zellij — managed by connect.sh (run by ttyd per browser connection).
#     First connection creates the session; subsequent ones attach. The Zellij
#     server persists independently of browser connections.
#   ttyd — web terminal that runs connect.sh per browser connection.
#   agent — Node.js sidecar for chat, file browsing, project management, and
#     creating Zellij project tabs.
set -u

MAX_BACKOFF=30

# --- ttyd (web terminal) ---
start_ttyd() {
    ttyd -W -p 7681 /home/claude/connect.sh &
    TTYD_PID=$!
    echo "supervisor: started ttyd (pid=$TTYD_PID)" >&2
}

# --- Agent (Node.js sidecar) ---
start_agent() {
    cd /opt/cloudcode/agent && node server.js &
    AGENT_PID=$!
    echo "supervisor: started agent (pid=$AGENT_PID)" >&2
}

# Track backoff per process
TTYD_BACKOFF=1
AGENT_BACKOFF=1

# Start processes
start_ttyd
start_agent

echo "supervisor: all processes started" >&2

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
