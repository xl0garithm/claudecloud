#!/bin/bash
# Connect to the persistent Zellij session — DEBUG VERSION
# Diagnostic output goes to the web terminal so we can see what's happening.

SESSION="main"
LAYOUT="/home/claude/.config/zellij/layouts/claude.kdl"

echo "=== connect.sh debug ==="
echo "zellij version: $(zellij --version 2>&1)"
echo "layout exists: $(test -f "${LAYOUT}" && echo YES || echo NO)"
echo "layout path: ${LAYOUT}"
echo "list-sessions output:"
zellij list-sessions 2>&1
echo "---"
echo "list-sessions exit code: $?"
echo "whoami: $(whoami)"
echo "tty: $(tty 2>&1)"
echo "TERM: ${TERM}"
echo "=== attempting session create ==="
echo "command: zellij -s ${SESSION} --layout ${LAYOUT}"
zellij -s "${SESSION}" --layout "${LAYOUT}" 2>&1
echo "=== zellij exited with code: $? ==="
echo "Sleeping 30s so you can read this..."
sleep 30
