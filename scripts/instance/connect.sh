#!/bin/bash
# Connect to the persistent Zellij session.
# Called by ttyd for each browser connection. The Zellij server is managed
# by supervisor.sh and runs independently — this script just attaches a client.
# When the browser disconnects, only this client exits; the server and all
# running processes (including Claude) continue working in the background.

SESSION="main"

# Wait for the session to be ready (race on container startup)
for i in $(seq 1 10); do
    if zellij list-sessions 2>/dev/null | grep -q "^${SESSION}"; then
        exec zellij attach "${SESSION}"
    fi
    sleep 1
done

# If session still doesn't exist after 10s, show an error
echo "Error: Zellij session '${SESSION}' not found. Check supervisor logs."
sleep 5
