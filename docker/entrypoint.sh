#!/bin/bash
set -e

# Authentication is handled interactively via the web UI's "Sign into Claude"
# button, which triggers Claude Code's OAuth flow in a background Zellij tab.
# Credentials persist in ~/.claude/.credentials.json on the volume across restarts.

# Delegate all process management to the supervisor
exec /supervisor.sh
