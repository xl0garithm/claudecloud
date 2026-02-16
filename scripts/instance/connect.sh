#!/bin/bash
# Connect to the persistent Zellij session — DEBUG VERSION 2
# Testing different approaches to create a session in Zellij 0.43.1

SESSION="main"
LAYOUT="/home/claude/.config/zellij/layouts/claude.kdl"

echo "=== Zellij 0.43.1 CLI tests ==="
echo ""

echo "--- Test 1: zellij --help (top-level flags) ---"
zellij --help 2>&1 | head -40
echo ""

echo "--- Test 2: zellij attach --help ---"
zellij attach --help 2>&1 | head -30
echo ""

echo "--- Test 3: zellij attach --create ${SESSION} (no layout) ---"
timeout 5 zellij attach --create "${SESSION}" 2>&1 || true
echo "(exit: $?)"
echo ""

echo "--- Test 4: plain zellij --layout ${LAYOUT} ---"
timeout 5 zellij --layout "${LAYOUT}" 2>&1 || true
echo "(exit: $?)"
echo ""

echo "Sleeping 60s so you can read this..."
sleep 60
