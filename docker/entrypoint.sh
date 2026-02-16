#!/bin/bash
set -e

# --- Configure Claude Code CLI credentials ---
# The SDK reads CLAUDE_CODE_OAUTH_TOKEN / ANTHROPIC_API_KEY from env vars,
# but the interactive CLI reads from ~/.claude/.credentials.json for OAuth.
# Write the credential file so both paths work.

CLAUDE_DIR="/home/claude/.claude"
CREDS_FILE="${CLAUDE_DIR}/.credentials.json"

if [ -n "${CLAUDE_CODE_OAUTH_TOKEN:-}" ]; then
    mkdir -p "${CLAUDE_DIR}"
    # Write OAuth credential file for the CLI
    # expiresAt far in the future (year 2035); no refresh token needed
    cat > "${CREDS_FILE}" <<CREDS
{"claudeAiOauth":{"accessToken":"${CLAUDE_CODE_OAUTH_TOKEN}","refreshToken":"","expiresAt":2051222400000}}
CREDS
    chmod 600 "${CREDS_FILE}"
    echo "entrypoint: configured Claude Code OAuth credentials" >&2
fi

# Delegate all process management to the supervisor
exec /supervisor.sh
