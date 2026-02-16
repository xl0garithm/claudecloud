# Terminal-Primary Architecture Design

## Decision

Drop the web chat UI as the user-facing interface. The Claude Code CLI running in the browser terminal is the primary (and only) way users interact with Claude. Zellij manages sessions invisibly under the hood. The Projects page becomes the control center for managing per-project Claude sessions.

## Goals

- Persistent Claude Code sessions that run autonomously in the background
- Sessions survive browser disconnects
- `--dangerously-skip-permissions` for fully autonomous work
- One session per project, with status visibility
- Zero Zellij learning curve — users only see Claude Code
- Conversation resume support via Claude Code's native `-c` / `--resume` flags

## Architecture

### Session Model

- One Zellij tab per project (named after the project)
- Each tab runs `claude --dangerously-skip-permissions` in the project's working directory
- Zellij server persists independently of browser connections
- The agent server manages tab lifecycle via Zellij IPC (`ZELLIJ_SESSION_NAME=main zellij action ...`)

### Zellij Configuration

Zellij is completely hidden from the user:

- `default_mode "locked"` — disables all Zellij keybindings
- `pane_frames false` — no pane borders
- `simplified_ui true` — minimal chrome
- Layout/theme with no tab bar or status bar
- Tab switching done entirely via the web UI (Projects page)

### Agent Endpoints

**Modified:**
- `POST /tabs` — Enhanced with `resume` (boolean) and `conversationId` (string) fields:
  - `resume: false` → `claude --dangerously-skip-permissions`
  - `resume: true` (no id) → `claude --dangerously-skip-permissions -c`
  - `resume: true, conversationId: "abc"` → `claude --dangerously-skip-permissions --resume abc`
  - Reuses existing tab (switches focus) instead of creating duplicates
  - Switches Zellij focus to the tab via `zellij action go-to-tab-name`

**New:**
- `GET /sessions` — Returns per-project session status:
  ```json
  [
    {"project": "my-app", "tab": "my-app", "status": "working", "cwd": "/claude-data/my-app"}
  ]
  ```
  Status detection: query Zellij tab names, check for foreground `claude` processes per tab.
  - `working` = claude process running as foreground process in the pane
  - `idle` = tab exists but claude has exited (shell prompt)
  - `none` = no tab for this project

- `GET /sessions/:project/conversations` — Lists recent conversations for resume picker. Runs `claude --resume --list` or scans `~/.claude/projects/` in the project directory.

- `DELETE /tabs/:name` — Kills the claude process and closes the Zellij tab.

**Auto-cleanup:**
- Agent monitors tabs every 60 seconds
- Tabs idle for >30 minutes (claude exited, shell unchanged) get closed automatically

### Projects Page (Control Center)

Each project card shows session status and provides contextual actions:

**No session:**
- "New Chat" button → creates tab, starts claude, navigates to terminal

**Working:**
- Green "Working" badge
- "Open Terminal" → navigates to terminal, switches focus to this tab
- "Stop" → kills claude process (tab stays, shows shell)

**Idle:**
- Gray "Idle" badge
- "Continue" → shows conversation picker modal (recent conversations from `GET /sessions/:project/conversations`), resumes selected conversation
- "New Chat" → warns that a session exists, confirms before killing and restarting

**Status polling:**
- Projects page polls `GET /sessions` every 5 seconds
- Agent caches Zellij state, only shells out once per poll cycle

### Terminal Page

- Renders Zellij via ttyd WebSocket (unchanged)
- Reads `?tab=<name>` query param to switch focus on load
- No other changes — user sees only Claude Code output

### Chat UI (Hidden)

- Remove "Chat" from dashboard sidebar navigation
- Remove "Open in Chat" button from ProjectCard
- Keep all chat code in the codebase (route, components, agent handler, DB tables) for future artifact viewer work

## Data Flow

```
User clicks "New Chat" on project "my-app"
  → POST /instances/{id}/tabs {name: "my-app", cwd: "my-app", resume: false}
  → Agent: creates Zellij tab, runs claude --dangerously-skip-permissions
  → router.push("/dashboard/terminal?tab=my-app")
  → User sees Claude Code running in their project

User navigates away, comes back later
  → Projects page: GET /instances/{id}/sessions
  → Agent: queries Zellij tabs + process state
  → Returns [{project: "my-app", status: "working"}]
  → UI shows green "Working" badge

User clicks "Continue" on idle project
  → GET /instances/{id}/sessions/my-app/conversations
  → UI shows conversation picker modal
  → User selects conversation "abc123"
  → POST /instances/{id}/tabs {name: "my-app", resume: true, conversationId: "abc123"}
  → Agent: reuses tab, starts claude --dangerously-skip-permissions --resume abc123
  → router.push("/dashboard/terminal?tab=my-app")
```

## Not in Scope (MVP)

- Artifact viewer / activity feed (future phase)
- Rich status parsing ("Editing file X") — just working/idle/none
- Multi-session per project
- Task assignment UI — users type prompts in the terminal
- Conversation history viewer in the web UI
- Changes to sudo/sandboxing (full sudo stays for MVP)
