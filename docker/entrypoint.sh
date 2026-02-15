#!/bin/bash
set -e

# Start a detached Zellij session named "claude"
zellij --session claude &

# Keep the container running
exec tail -f /dev/null
