#!/bin/bash
# Process supervisor for Claude instance containers.
#
# Architecture:
#   Zellij server — persistent session daemon. Claude Code runs inside Zellij
#     panes and keeps working even when no browser is connected. The server is
#     started headlessly via `script` (which provides a PTY) and persists as a
#     daemon after the initial client exits.
#   ttyd — web terminal that attaches a Zellij client per browser connection.
#     Disconnecting only kills the client; the server and all processes persist.
#   agent — Node.js sidecar for chat, file browsing, project management, and
#     creating Zellij project tabs.
set -u

MAX_BACKOFF=30
SESSION="main"
LAYOUT="/home/claude/.config/zellij/layouts/claude.kdl"

# --- Zellij session (persistent daemon) ---
start_zellij() {
    echo "supervisor: creating Zellij session '${SESSION}'" >&2
    # Use `script` to provide a PTY for headless Zellij startup.
    # Zellij forks a server daemon, then the client blocks. After a brief
    # delay we kill the wrapper, leaving the server running independently.
    script -qfc "zellij --session ${SESSION} --layout ${LAYOUT}" /dev/null &
    local wrapper_pid=$!

    # Wait for the session to appear
    for i in $(seq 1 10); do
        if zellij list-sessions 2>/dev/null | grep -q "^${SESSION}"; then
            echo "supervisor: Zellij session '${SESSION}' is running" >&2
            # Kill the wrapper client — server stays alive
            kill "$wrapper_pid" 2>/dev/null
            wait "$wrapper_pid" 2>/dev/null
            return 0
        fi
        sleep 1
    done

    # Fallback: kill wrapper and report failure
    kill "$wrapper_pid" 2>/dev/null
    wait "$wrapper_pid" 2>/dev/null
    echo "supervisor: WARNING — Zellij session failed to start" >&2
    return 1
}

zellij_running() {
    zellij list-sessions 2>/dev/null | grep -q "^${SESSION}"
}

# --- ttyd (web terminal viewer) ---
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
ZELLIJ_BACKOFF=1

# Start all processes (Zellij first — ttyd depends on it)
start_zellij
start_ttyd
start_agent

# Monitor loop
while true; do
    sleep 5

    # Check Zellij server (via session list, not PID)
    if ! zellij_running; then
        echo "supervisor: Zellij session gone, restarting (backoff=${ZELLIJ_BACKOFF}s)" >&2
        sleep "$ZELLIJ_BACKOFF"
        start_zellij
        ZELLIJ_BACKOFF=$((ZELLIJ_BACKOFF * 2))
        if [ "$ZELLIJ_BACKOFF" -gt "$MAX_BACKOFF" ]; then
            ZELLIJ_BACKOFF=$MAX_BACKOFF
        fi
    else
        ZELLIJ_BACKOFF=1
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
