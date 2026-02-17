#!/bin/bash
# Process supervisor for Claude instance containers.
#
# Architecture:
#   Zellij — session pre-created in background at startup. The server persists
#     independently of browser connections. ttyd's connect.sh attaches clients.
#   ttyd — web terminal that runs connect.sh per browser connection.
#   agent — Node.js sidecar for chat, file browsing, project management, and
#     creating Zellij project tabs via IPC.
set -u

MAX_BACKOFF=30
SESSION="main"

# --- Zellij (background session) ---
start_zellij() {
    # Pre-create the session in the background so it's ready before any
    # browser connects or the agent tries to create project tabs.
    # Uses --create-background: creates a detached session if it doesn't exist.
    if ! zellij list-sessions 2>/dev/null | sed 's/\x1b\[[0-9;]*m//g' | grep -q "${SESSION}"; then
        zellij attach --create-background "${SESSION}" 2>&1 || true
        echo "supervisor: created Zellij session '${SESSION}'" >&2
    else
        echo "supervisor: Zellij session '${SESSION}' already exists" >&2
    fi
}

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

# Start Zellij session first (agent needs it for tab creation)
start_zellij
start_ttyd
start_agent

echo "supervisor: all processes started" >&2

# Monitor loop
while true; do
    sleep 5

    # Check Zellij session is still alive (strip ANSI codes from list-sessions output)
    if ! zellij list-sessions 2>/dev/null | sed 's/\x1b\[[0-9;]*m//g' | grep -q "${SESSION}"; then
        echo "supervisor: Zellij session gone, recreating" >&2
        start_zellij
    fi

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
