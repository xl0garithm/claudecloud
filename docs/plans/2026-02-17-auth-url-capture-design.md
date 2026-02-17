# Auth URL Capture Design

**Date:** 2026-02-17
**Status:** Approved

## Problem

Claude Code's OAuth token written by the entrypoint (`~/.claude/.credentials.json`) isn't being recognized, forcing users to authenticate manually through the terminal. The terminal-based auth flow is clunky through ttyd. Users need a smoother way to authenticate Claude Code on their cloud instance.

## Solution

Capture the authentication URL that Claude Code displays when it needs login, and surface it as a clickable button in the web UI. Auth is a gate — the terminal is inaccessible until authentication completes.

## Architecture

### Agent (server.js)

**Startup auth check:**
1. Check if `~/.claude/.credentials.json` exists with a valid token
2. If authenticated: set `authState = { status: "authenticated" }` — done
3. If not authenticated: create a Zellij tab `_auth` running `claude`, start polling

**Polling loop (every 2 seconds):**
- `zellij -s main action dump-screen /tmp/auth-dump.txt` on the `_auth` tab
- Read dump file, regex-match for `https://` auth URLs
- Store URL in `authState = { status: "awaiting_auth", url: "https://..." }`
- Check if `~/.claude/.credentials.json` appeared (auth completed)
- On completion: set `authState = { status: "authenticated" }`, close `_auth` tab, stop polling

**New endpoint:**
- `GET /auth/status` → `{ status: "authenticated" | "awaiting_auth" | "checking", url?: string }`

**Convention:** Tabs prefixed with `_` are internal and excluded from `/sessions` results.

### Go Proxy (proxy.go + server.go)

One new route:
- `GET /instances/{id}/auth/status` → proxies to `GET /auth/status`

### Frontend

**Terminal page (gate):**
- Poll `GET /instances/{id}/auth/status` every 3s
- `authenticated` → render terminal normally
- `awaiting_auth` → render auth card: "Claude Code needs authentication" + "Sign into Claude" button (opens URL in new tab) + "Waiting for authentication..."
- `checking` → loading state

**Projects page:**
- If not authenticated, disable "New Chat" buttons
- Show banner: "Sign in to Claude to start using your instance" with URL button

## Edge Cases

- **Token valid from env var:** Credentials file exists on startup → auth flow skipped entirely
- **No timeout:** `_auth` tab stays until user completes auth; polling is lightweight
- **Instance restart:** Auth state derived from credentials file, persists across restarts
- **Re-auth mid-session:** Not handled by this feature; Claude prompts in terminal directly
- **URL regex:** Broad `https://` match, forward-compatible with domain changes
