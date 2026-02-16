# Terminal-Primary Architecture Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the Claude Code CLI in the browser terminal the only user-facing interface, with the Projects page as the session control center.

**Architecture:** Zellij runs invisibly (locked mode, no frames/bars). One Zellij tab per project, managed via agent IPC. Projects page shows session status (working/idle/none) and provides New Chat / Continue / Stop actions. Chat UI hidden from navigation but code preserved.

**Tech Stack:** Node.js (instance agent), Go (API proxy), Next.js (dashboard), Zellij (terminal multiplexer)

---

### Task 1: Hide Zellij UI

**Files:**
- Modify: `scripts/instance/zellij-config.kdl`
- Modify: `scripts/instance/claude-layout.kdl`

**Step 1: Update Zellij config to hide all UI**

Replace `scripts/instance/zellij-config.kdl` with locked-down config:

```kdl
// Zellij configuration for Claude Code instances.
// Zellij is invisible — users only see Claude Code output.

default_layout "claude"
default_mode "locked"
pane_frames false
simplified_ui true

keybinds clear-defaults=true {
}

themes {
    default {
        bg "#000000"
        fg "#000000"
    }
}

ui {
    pane_frames {
        hide_session_name true
    }
}
```

**Step 2: Simplify layout to single pane (no shell split)**

Replace `scripts/instance/claude-layout.kdl`:

```kdl
// Zellij layout for Claude Code instances.
// Single pane — tab switching managed by the web UI via agent IPC.
layout {
    tab name="shell" focus=true {
        pane
    }
}
```

The default tab is a plain shell. Per-project tabs are created dynamically by POST /tabs.

**Step 3: Verify**

Rebuild the instance image and confirm: no tab bar, no status bar, no pane frames, no keybindings respond.

**Step 4: Commit**

```bash
git add scripts/instance/zellij-config.kdl scripts/instance/claude-layout.kdl
git commit -m "feat: hide Zellij UI completely (locked mode, no frames/bars)"
```

---

### Task 2: Agent GET /sessions endpoint

**Files:**
- Modify: `scripts/instance/agent/server.js`

**Step 1: Add GET /sessions route**

Add after the existing `/tabs` endpoint in `server.js`:

```javascript
// --- GET /sessions — Per-project session status ---
app.get("/sessions", (req, res) => {
  const session = "main";

  // Get tab names from Zellij
  execFile(
    "zellij",
    ["action", "query-tab-names"],
    { env: { ...process.env, ZELLIJ_SESSION_NAME: session }, timeout: 5000 },
    (err, stdout) => {
      if (err) {
        return res.json([]); // No Zellij session running
      }

      const tabNames = stdout.trim().split("\n").filter(Boolean);
      // Skip the default "shell" tab
      const projectTabs = tabNames.filter((t) => t !== "shell");

      if (projectTabs.length === 0) {
        return res.json([]);
      }

      // For each tab, check if claude is running as foreground process
      const sessions = [];
      let pending = projectTabs.length;

      projectTabs.forEach((tabName) => {
        // Use pgrep to check if claude is running in the tab
        // We check for any claude process with the project's cwd
        const projectCwd = path.join(DATA_ROOT, tabName);
        execFile(
          "pgrep",
          ["-f", `claude.*${tabName}`],
          { timeout: 3000 },
          (err) => {
            sessions.push({
              project: tabName,
              tab: tabName,
              status: err ? "idle" : "working",
              cwd: projectCwd,
            });
            pending--;
            if (pending === 0) {
              res.json(sessions);
            }
          }
        );
      });
    }
  );
});
```

**Step 2: Verify**

Test manually: `curl -H "Authorization: Bearer $AGENT_SECRET" http://localhost:3001/sessions`

**Step 3: Commit**

```bash
git add scripts/instance/agent/server.js
git commit -m "feat: add GET /sessions endpoint for per-project session status"
```

---

### Task 3: Agent enhanced POST /tabs with resume support

**Files:**
- Modify: `scripts/instance/agent/server.js`

**Step 1: Rewrite the POST /tabs handler**

Replace the existing `POST /tabs` handler with resume-aware version:

```javascript
// --- POST /tabs — Create or resume a Claude Code session in a Zellij tab ---
app.post("/tabs", (req, res) => {
  const { name, cwd, resume, conversationId } = req.body;
  if (!name || typeof name !== "string") {
    return res.status(400).json({ error: "name is required" });
  }

  const tabCwd = safePath(cwd || name) || DATA_ROOT;
  const session = "main";
  const zellijEnv = { ...process.env, ZELLIJ_SESSION_NAME: session };

  // Build the claude command based on resume options
  let claudeCmd = "claude --dangerously-skip-permissions";
  if (resume && conversationId) {
    claudeCmd += ` --resume ${conversationId}`;
  } else if (resume) {
    claudeCmd += " -c";
  }

  // Check if a tab with this name already exists
  execFile(
    "zellij",
    ["action", "query-tab-names"],
    { env: zellijEnv, timeout: 5000 },
    (err, stdout) => {
      const tabNames = (!err && stdout) ? stdout.trim().split("\n") : [];
      const tabExists = tabNames.includes(name);

      if (tabExists) {
        // Switch focus to the existing tab
        execFile(
          "zellij",
          ["action", "go-to-tab-name", name],
          { env: zellijEnv, timeout: 5000 },
          () => {
            // Write the claude command into the pane (user may want to resume)
            if (resume) {
              execFile(
                "zellij",
                ["action", "write-chars", claudeCmd + "\n"],
                { env: zellijEnv, timeout: 5000 },
                () => {}
              );
            }
            res.json({ status: "exists", tab: name });
          }
        );
        return;
      }

      // Ensure the project directory exists
      fs.mkdir(tabCwd, { recursive: true }, () => {
        // Create a new tab
        execFile(
          "zellij",
          ["action", "new-tab", "--name", name, "--cwd", tabCwd],
          { env: zellijEnv, timeout: 5000 },
          (err) => {
            if (err) {
              return res.status(500).json({ error: "failed to create tab: " + err.message });
            }

            // Start claude in the new tab
            execFile(
              "zellij",
              ["action", "write-chars", claudeCmd + "\n"],
              { env: zellijEnv, timeout: 5000 },
              () => {}
            );

            res.json({ status: "created", tab: name });
          }
        );
      });
    }
  );
});
```

**Step 2: Verify**

Test: `curl -X POST -H "Authorization: Bearer $AGENT_SECRET" -H "Content-Type: application/json" -d '{"name":"test","resume":false}' http://localhost:3001/tabs`

**Step 3: Commit**

```bash
git add scripts/instance/agent/server.js
git commit -m "feat: enhance POST /tabs with resume and conversationId support"
```

---

### Task 4: Agent DELETE /tabs/:name endpoint

**Files:**
- Modify: `scripts/instance/agent/server.js`

**Step 1: Add DELETE /tabs/:name route**

```javascript
// --- DELETE /tabs/:name — Kill claude process and close Zellij tab ---
app.delete("/tabs/:name", (req, res) => {
  const { name } = req.params;
  const session = "main";
  const zellijEnv = { ...process.env, ZELLIJ_SESSION_NAME: session };

  // Switch to the tab first, then close it
  execFile(
    "zellij",
    ["action", "go-to-tab-name", name],
    { env: zellijEnv, timeout: 5000 },
    (err) => {
      if (err) {
        return res.status(404).json({ error: "tab not found" });
      }

      // Close the tab (kills all panes/processes in it)
      execFile(
        "zellij",
        ["action", "close-tab"],
        { env: zellijEnv, timeout: 5000 },
        (err) => {
          if (err) {
            return res.status(500).json({ error: "failed to close tab: " + err.message });
          }
          res.json({ status: "closed", tab: name });
        }
      );
    }
  );
});
```

**Step 2: Verify**

Test: `curl -X DELETE -H "Authorization: Bearer $AGENT_SECRET" http://localhost:3001/tabs/test`

**Step 3: Commit**

```bash
git add scripts/instance/agent/server.js
git commit -m "feat: add DELETE /tabs/:name to kill sessions and close tabs"
```

---

### Task 5: Agent GET /sessions/:project/conversations

**Files:**
- Modify: `scripts/instance/agent/server.js`

**Step 1: Add conversations listing endpoint**

```javascript
// --- GET /sessions/:project/conversations — List recent conversations for resume ---
app.get("/sessions/:project/conversations", (req, res) => {
  const { project } = req.params;
  const projectDir = safePath(project);
  if (!projectDir) {
    return res.status(400).json({ error: "invalid project" });
  }

  // Scan ~/.claude/projects/ for conversation data
  // Claude Code stores project data keyed by absolute path
  const claudeProjectsDir = path.join("/home/claude/.claude/projects");

  fs.readdir(claudeProjectsDir, { withFileTypes: true }, (err, entries) => {
    if (err) {
      return res.json([]); // No conversations yet
    }

    // Find directory matching this project path
    // Claude Code encodes the path as the directory name
    const conversations = [];
    const encodedPath = projectDir.replace(/\//g, "-");

    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      // Check if this directory name contains our project path
      if (entry.name.includes(project) || entry.name.includes(encodedPath)) {
        // Scan for conversation files
        const convDir = path.join(claudeProjectsDir, entry.name);
        try {
          const files = fs.readdirSync(convDir);
          for (const file of files) {
            if (file.endsWith(".json")) {
              try {
                const data = JSON.parse(
                  fs.readFileSync(path.join(convDir, file), "utf-8")
                );
                conversations.push({
                  id: path.basename(file, ".json"),
                  title: data.title || data.name || file,
                  updatedAt: data.updatedAt || data.timestamp || null,
                });
              } catch {
                // Skip malformed files
              }
            }
          }
        } catch {
          // Skip unreadable dirs
        }
      }
    }

    // Sort by most recent first
    conversations.sort((a, b) => {
      if (!a.updatedAt) return 1;
      if (!b.updatedAt) return -1;
      return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
    });

    res.json(conversations.slice(0, 20)); // Limit to 20 most recent
  });
});
```

**Step 2: Verify**

Test: `curl -H "Authorization: Bearer $AGENT_SECRET" http://localhost:3001/sessions/my-app/conversations`

**Step 3: Commit**

```bash
git add scripts/instance/agent/server.js
git commit -m "feat: add GET /sessions/:project/conversations for resume picker"
```

---

### Task 6: Agent auto-cleanup loop

**Files:**
- Modify: `scripts/instance/agent/server.js`

**Step 1: Add idle tab tracking and cleanup**

Add at the bottom of server.js, before `server.listen()`:

```javascript
// --- Auto-cleanup: close idle tabs after 30 minutes ---
const idleSince = new Map(); // tabName → timestamp

function cleanupIdleTabs() {
  const session = "main";
  const zellijEnv = { ...process.env, ZELLIJ_SESSION_NAME: session };
  const IDLE_TIMEOUT_MS = 30 * 60 * 1000; // 30 minutes

  execFile(
    "zellij",
    ["action", "query-tab-names"],
    { env: zellijEnv, timeout: 5000 },
    (err, stdout) => {
      if (err) return;

      const tabNames = stdout.trim().split("\n").filter(Boolean);
      const projectTabs = tabNames.filter((t) => t !== "shell");
      const now = Date.now();

      projectTabs.forEach((tabName) => {
        // Check if claude is running
        execFile("pgrep", ["-f", `claude.*${tabName}`], { timeout: 3000 }, (err) => {
          if (!err) {
            // Claude is running — not idle
            idleSince.delete(tabName);
            return;
          }

          // Claude not running — track idle time
          if (!idleSince.has(tabName)) {
            idleSince.set(tabName, now);
            return;
          }

          const idleDuration = now - idleSince.get(tabName);
          if (idleDuration >= IDLE_TIMEOUT_MS) {
            console.log(`auto-cleanup: closing idle tab "${tabName}" (idle ${Math.round(idleDuration / 60000)}m)`);
            // Switch to tab and close it
            execFile(
              "zellij",
              ["action", "go-to-tab-name", tabName],
              { env: zellijEnv, timeout: 5000 },
              () => {
                execFile(
                  "zellij",
                  ["action", "close-tab"],
                  { env: zellijEnv, timeout: 5000 },
                  () => {
                    idleSince.delete(tabName);
                  }
                );
              }
            );
          }
        });
      });

      // Clean up tracking for tabs that no longer exist
      for (const tracked of idleSince.keys()) {
        if (!projectTabs.includes(tracked)) {
          idleSince.delete(tracked);
        }
      }
    }
  );
}

setInterval(cleanupIdleTabs, 60 * 1000); // Check every 60 seconds
```

**Step 2: Verify**

Check logs for auto-cleanup messages after creating and abandoning a tab.

**Step 3: Commit**

```bash
git add scripts/instance/agent/server.js
git commit -m "feat: add auto-cleanup loop for idle Zellij tabs (30min timeout)"
```

---

### Task 7: Go proxy routes for new agent endpoints

**Files:**
- Modify: `internal/api/handler/proxy.go`
- Modify: `internal/api/server.go`

**Step 1: Add proxy methods for Sessions, SessionConversations, and DeleteTab**

Add to `internal/api/handler/proxy.go`:

```go
// Sessions proxies GET /instances/{id}/sessions to the agent.
func (h *ProxyHandler) Sessions(w http.ResponseWriter, r *http.Request) {
	h.proxyHTTP(w, r, "/sessions")
}

// SessionConversations proxies GET /instances/{id}/sessions/{project}/conversations to the agent.
func (h *ProxyHandler) SessionConversations(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	h.proxyHTTP(w, r, "/sessions/"+project+"/conversations")
}

// DeleteTab proxies DELETE /instances/{id}/tabs/{name} to the agent.
func (h *ProxyHandler) DeleteTab(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	h.proxyHTTP(w, r, "/tabs/"+name)
}
```

**Step 2: Register routes in `internal/api/server.go`**

Add inside the `/instances` route group, after the existing proxy routes:

```go
r.Get("/{id}/sessions", proxyH.Sessions)
r.Get("/{id}/sessions/{project}/conversations", proxyH.SessionConversations)
r.Delete("/{id}/tabs/{name}", proxyH.DeleteTab)
```

**Step 3: Verify**

Run: `go build ./...`

**Step 4: Commit**

```bash
git add internal/api/handler/proxy.go internal/api/server.go
git commit -m "feat: add Go proxy routes for sessions, conversations, and tab deletion"
```

---

### Task 8: Frontend API methods

**Files:**
- Modify: `web/lib/api.ts`

**Step 1: Add new types and API methods**

Add types:

```typescript
export interface SessionInfo {
  project: string;
  tab: string;
  status: "working" | "idle" | "none";
  cwd: string;
}

export interface ConversationInfo {
  id: string;
  title: string;
  updatedAt: string | null;
}
```

Add methods to the `api` object:

```typescript
  // Sessions
  getSessions(instanceId: number) {
    return apiFetch<SessionInfo[]>(`/instances/${instanceId}/sessions`);
  },

  getSessionConversations(instanceId: number, project: string) {
    return apiFetch<ConversationInfo[]>(
      `/instances/${instanceId}/sessions/${encodeURIComponent(project)}/conversations`
    );
  },

  deleteTab(instanceId: number, name: string) {
    return apiFetch<{ status: string; tab: string }>(
      `/instances/${instanceId}/tabs/${encodeURIComponent(name)}`,
      { method: "DELETE" }
    );
  },
```

Update the `createTab` signature to accept resume params:

```typescript
  createTab(instanceId: number, name: string, cwd?: string, options?: { resume?: boolean; conversationId?: string }) {
    return apiFetch<{ status: string; tab: string }>(
      `/instances/${instanceId}/tabs`,
      {
        method: "POST",
        body: JSON.stringify({
          name,
          cwd: cwd || undefined,
          resume: options?.resume,
          conversationId: options?.conversationId,
        }),
      }
    );
  },
```

**Step 2: Verify**

Run: `cd web && npx tsc --noEmit`

**Step 3: Commit**

```bash
git add web/lib/api.ts
git commit -m "feat: add frontend API methods for sessions, conversations, tab deletion"
```

---

### Task 9: Redesign Projects page as session control center

**Files:**
- Modify: `web/components/ProjectCard.tsx`
- Modify: `web/app/dashboard/projects/page.tsx`

**Step 1: Rewrite ProjectCard with session status and actions**

Replace `web/components/ProjectCard.tsx`:

```tsx
"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { api, Project, Instance, SessionInfo, ConversationInfo } from "@/lib/api";

interface ProjectCardProps {
  project: Project;
  instance: Instance;
  session?: SessionInfo;
}

export default function ProjectCard({ project, instance, session }: ProjectCardProps) {
  const router = useRouter();
  const [acting, setActing] = useState(false);
  const [showConversations, setShowConversations] = useState(false);
  const [conversations, setConversations] = useState<ConversationInfo[]>([]);
  const [showConfirm, setShowConfirm] = useState(false);

  const status = session?.status || "none";

  async function handleNewChat() {
    if (status === "idle" || status === "working") {
      setShowConfirm(true);
      return;
    }
    await startNewChat();
  }

  async function startNewChat() {
    setActing(true);
    setShowConfirm(false);
    try {
      await api.createTab(instance.id, project.name, project.path, { resume: false });
      router.push(`/dashboard/terminal?tab=${encodeURIComponent(project.name)}`);
    } catch {
      router.push("/dashboard/terminal");
    } finally {
      setActing(false);
    }
  }

  async function handleContinue() {
    setActing(true);
    try {
      const convs = await api.getSessionConversations(instance.id, project.name);
      if (convs.length === 0) {
        // No conversations — just resume last
        await api.createTab(instance.id, project.name, project.path, { resume: true });
        router.push(`/dashboard/terminal?tab=${encodeURIComponent(project.name)}`);
      } else {
        setConversations(convs);
        setShowConversations(true);
      }
    } catch {
      // Fallback: resume last
      await api.createTab(instance.id, project.name, project.path, { resume: true });
      router.push(`/dashboard/terminal?tab=${encodeURIComponent(project.name)}`);
    } finally {
      setActing(false);
    }
  }

  async function handleResumeConversation(convId: string) {
    setActing(true);
    setShowConversations(false);
    try {
      await api.createTab(instance.id, project.name, project.path, {
        resume: true,
        conversationId: convId,
      });
      router.push(`/dashboard/terminal?tab=${encodeURIComponent(project.name)}`);
    } catch {
      router.push("/dashboard/terminal");
    } finally {
      setActing(false);
    }
  }

  async function handleOpenTerminal() {
    setActing(true);
    try {
      await api.createTab(instance.id, project.name, project.path);
      router.push(`/dashboard/terminal?tab=${encodeURIComponent(project.name)}`);
    } catch {
      router.push("/dashboard/terminal");
    } finally {
      setActing(false);
    }
  }

  async function handleStop() {
    setActing(true);
    try {
      await api.deleteTab(instance.id, project.name);
    } catch {
      // best-effort
    } finally {
      setActing(false);
    }
  }

  return (
    <div className="rounded-lg bg-white p-4 ring-1 ring-gray-200">
      <div className="flex items-start justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h3 className="font-semibold text-gray-900">{project.name}</h3>
            {status === "working" && (
              <span className="inline-flex items-center rounded-full bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700 ring-1 ring-green-600/20">
                Working
              </span>
            )}
            {status === "idle" && (
              <span className="inline-flex items-center rounded-full bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600 ring-1 ring-gray-500/20">
                Idle
              </span>
            )}
          </div>
          <p className="mt-1 text-xs text-gray-500">/{project.path}</p>
          {project.remoteUrl && (
            <p className="mt-1 truncate text-xs text-gray-400 max-w-sm">
              {project.remoteUrl}
            </p>
          )}
        </div>

        <div className="flex gap-2 ml-4">
          {status === "none" && (
            <button
              onClick={handleNewChat}
              disabled={acting}
              className="rounded bg-gray-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-gray-800 disabled:opacity-50"
            >
              {acting ? "Starting..." : "New Chat"}
            </button>
          )}

          {status === "working" && (
            <>
              <button
                onClick={handleOpenTerminal}
                disabled={acting}
                className="rounded bg-gray-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-gray-800 disabled:opacity-50"
              >
                Open Terminal
              </button>
              <button
                onClick={handleStop}
                disabled={acting}
                className="rounded bg-red-50 px-3 py-1.5 text-xs font-medium text-red-700 hover:bg-red-100 disabled:opacity-50"
              >
                Stop
              </button>
            </>
          )}

          {status === "idle" && (
            <>
              <button
                onClick={handleContinue}
                disabled={acting}
                className="rounded bg-gray-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-gray-800 disabled:opacity-50"
              >
                {acting ? "Loading..." : "Continue"}
              </button>
              <button
                onClick={handleNewChat}
                disabled={acting}
                className="rounded bg-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-300 disabled:opacity-50"
              >
                New Chat
              </button>
            </>
          )}
        </div>
      </div>

      {/* Confirm dialog for overwriting active session */}
      {showConfirm && (
        <div className="mt-3 rounded-lg bg-amber-50 p-3 ring-1 ring-amber-200">
          <p className="text-sm text-amber-800">
            This project has an active session. Starting a new chat will end it.
          </p>
          <div className="mt-2 flex gap-2">
            <button
              onClick={startNewChat}
              className="rounded bg-amber-600 px-3 py-1 text-xs font-medium text-white hover:bg-amber-700"
            >
              Confirm
            </button>
            <button
              onClick={() => setShowConfirm(false)}
              className="rounded bg-white px-3 py-1 text-xs font-medium text-gray-700 ring-1 ring-gray-300 hover:bg-gray-50"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Conversation picker modal */}
      {showConversations && (
        <div className="mt-3 rounded-lg bg-gray-50 p-3 ring-1 ring-gray-200">
          <p className="mb-2 text-sm font-medium text-gray-700">
            Resume a conversation:
          </p>
          <div className="max-h-48 space-y-1 overflow-y-auto">
            {conversations.map((conv) => (
              <button
                key={conv.id}
                onClick={() => handleResumeConversation(conv.id)}
                className="block w-full rounded px-2 py-1.5 text-left text-sm text-gray-700 hover:bg-gray-200"
              >
                <span className="font-medium">{conv.title}</span>
                {conv.updatedAt && (
                  <span className="ml-2 text-xs text-gray-500">
                    {new Date(conv.updatedAt).toLocaleDateString()}
                  </span>
                )}
              </button>
            ))}
          </div>
          <button
            onClick={() => setShowConversations(false)}
            className="mt-2 text-xs text-gray-500 hover:text-gray-700"
          >
            Cancel
          </button>
        </div>
      )}
    </div>
  );
}
```

**Step 2: Update Projects page with session polling**

Replace `web/app/dashboard/projects/page.tsx`:

```tsx
"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api, Instance, Project, SessionInfo } from "@/lib/api";
import ProjectCard from "@/components/ProjectCard";
import CloneRepoForm from "@/components/CloneRepoForm";

export default function ProjectsPage() {
  const [instance, setInstance] = useState<Instance | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval>>();

  const loadProjects = useCallback(async (inst: Instance) => {
    try {
      const projs = await api.getProjects(inst.id);
      setProjects(projs);
    } catch {
      // Instance may not be running
    }
  }, []);

  const loadSessions = useCallback(async (inst: Instance) => {
    try {
      const sess = await api.getSessions(inst.id);
      setSessions(sess);
    } catch {
      // Ignore polling errors
    }
  }, []);

  useEffect(() => {
    api
      .getMyInstance()
      .then((inst) => {
        setInstance(inst);
        if (inst.status === "running") {
          loadProjects(inst);
          loadSessions(inst);
          // Poll sessions every 5 seconds
          pollRef.current = setInterval(() => loadSessions(inst), 5000);
        }
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load instance");
      })
      .finally(() => setLoading(false));

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [loadProjects, loadSessions]);

  async function handleClone(url: string, branch?: string) {
    if (!instance) return;
    await api.cloneProject(instance.id, url, branch);
    await loadProjects(instance);
  }

  function getSessionForProject(project: Project): SessionInfo | undefined {
    return sessions.find((s) => s.project === project.name);
  }

  if (loading) {
    return (
      <div className="flex h-[calc(100vh-8rem)] items-center justify-center">
        <div className="text-gray-500">Loading instance...</div>
      </div>
    );
  }

  if (error || !instance) {
    return (
      <div className="flex h-[calc(100vh-8rem)] items-center justify-center">
        <div className="text-center">
          <p className="text-gray-600">{error || "No active instance found."}</p>
          <p className="mt-2 text-sm text-gray-500">
            Provision an instance from the Overview tab first.
          </p>
        </div>
      </div>
    );
  }

  if (instance.status !== "running") {
    return (
      <div className="flex h-[calc(100vh-8rem)] items-center justify-center">
        <div className="text-center">
          <p className="text-gray-600">
            Instance is {instance.status}. Wake it from the Overview tab to
            manage projects.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <CloneRepoForm onClone={handleClone} />

      <div>
        <h2 className="mb-4 text-lg font-semibold">Projects</h2>
        {projects.length === 0 ? (
          <div className="rounded-lg bg-white p-8 text-center ring-1 ring-gray-200">
            <p className="text-gray-600">No projects yet.</p>
            <p className="mt-1 text-sm text-gray-500">
              Clone a repository above to get started.
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {projects.map((project) => (
              <ProjectCard
                key={project.path}
                project={project}
                instance={instance}
                session={getSessionForProject(project)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
```

**Step 3: Verify**

Run: `cd web && npm run build`

**Step 4: Commit**

```bash
git add web/components/ProjectCard.tsx web/app/dashboard/projects/page.tsx
git commit -m "feat: redesign Projects page as session control center with status badges"
```

---

### Task 10: Terminal page tab switching

**Files:**
- Modify: `web/app/dashboard/terminal/page.tsx`

**Step 1: Read ?tab= query param and switch Zellij focus**

Replace `web/app/dashboard/terminal/page.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { api, Instance } from "@/lib/api";
import WebTerminal from "@/components/WebTerminal";

export default function TerminalPage() {
  const searchParams = useSearchParams();
  const tab = searchParams.get("tab");
  const [instance, setInstance] = useState<Instance | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .getMyInstance()
      .then((inst) => {
        setInstance(inst);
        // If a tab param is provided, switch Zellij focus to it
        if (tab && inst.status === "running") {
          api.createTab(inst.id, tab, tab).catch(() => {
            // Best-effort tab switch
          });
        }
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load instance");
      })
      .finally(() => setLoading(false));
  }, [tab]);

  if (loading) {
    return (
      <div className="flex h-[calc(100vh-8rem)] items-center justify-center">
        <div className="text-gray-500">Loading instance...</div>
      </div>
    );
  }

  if (error || !instance) {
    return (
      <div className="flex h-[calc(100vh-8rem)] items-center justify-center">
        <div className="text-center">
          <p className="text-gray-600">
            {error || "No active instance found."}
          </p>
          <p className="mt-2 text-sm text-gray-500">
            Provision an instance from the Overview tab first.
          </p>
        </div>
      </div>
    );
  }

  if (instance.status !== "running") {
    return (
      <div className="flex h-[calc(100vh-8rem)] items-center justify-center">
        <div className="text-center">
          <p className="text-gray-600">
            Instance is {instance.status}. Wake it from the Overview tab to
            connect.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-[calc(100vh-8rem)] overflow-hidden rounded-lg ring-1 ring-gray-200">
      <WebTerminal instanceId={instance.id} />
    </div>
  );
}
```

**Step 2: Verify**

Run: `cd web && npm run build`

**Step 3: Commit**

```bash
git add web/app/dashboard/terminal/page.tsx
git commit -m "feat: terminal page reads ?tab= param to switch Zellij focus"
```

---

### Task 11: Hide Chat from navigation

**Files:**
- Modify: `web/app/dashboard/layout.tsx`

**Step 1: Remove Chat tab and reorder navigation**

In `web/app/dashboard/layout.tsx`, change `navTabs` to:

```typescript
const navTabs = [
  { label: "Overview", href: "/dashboard" },
  { label: "Projects", href: "/dashboard/projects" },
  { label: "Terminal", href: "/dashboard/terminal" },
  { label: "Settings", href: "/dashboard/settings" },
];
```

Projects is promoted above Terminal since it's now the primary control center.

**Step 2: Verify**

Run: `cd web && npm run build`

**Step 3: Commit**

```bash
git add web/app/dashboard/layout.tsx
git commit -m "feat: hide Chat from navigation, promote Projects as primary tab"
```

---

### Task 12: Final verification

**Step 1: Run Go build + tests**

```bash
go build ./... && go test ./... -short
```

**Step 2: Run frontend build**

```bash
cd web && npm run build
```

**Step 3: Rebuild Docker instance image**

```bash
docker build -f docker/Dockerfile.instance -t claude-instance:latest .
```

**Step 4: End-to-end verification checklist**

- [ ] Zellij has no visible UI (no tab bar, status bar, frames, keybindings)
- [ ] Projects page shows Working/Idle badges
- [ ] "New Chat" creates a tab and starts claude --dangerously-skip-permissions
- [ ] "Continue" shows conversation picker
- [ ] "Stop" kills the session
- [ ] "Open Terminal" navigates with ?tab= param
- [ ] Terminal page switches Zellij focus based on ?tab=
- [ ] Chat is gone from navigation
- [ ] Idle tabs auto-close after 30 minutes

**Step 5: Commit everything**

```bash
git add -A
git commit -m "feat: terminal-primary architecture — complete implementation"
```
