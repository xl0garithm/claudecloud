# Auth URL Capture Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Capture Claude Code's OAuth URL from a background Zellij tab and surface it in the web UI, gating the terminal until auth completes.

**Architecture:** Agent checks credentials on startup, spawns a hidden `_auth` Zellij tab if unauthenticated, polls `dump-screen` for the auth URL, and exposes `GET /auth/status`. Go proxy adds one route. Frontend gates terminal page and disables project actions behind auth status.

**Tech Stack:** Node.js (agent), Go (proxy), Next.js/React (frontend)

---

### Task 1: Agent — Auth state module and credential check

**Files:**
- Modify: `scripts/instance/agent/server.js:24-26` (add constants)
- Modify: `scripts/instance/agent/server.js:461-463` (startup hook after listen)

**Step 1: Add auth constants and state after the existing constants block (line 26)**

After `const DATA_ROOT = "/claude-data";` add:

```javascript
// --- Auth state ---
const CREDS_PATH = "/home/claude/.claude/.credentials.json";
const AUTH_TAB = "_auth";
let authState = { status: "checking", url: null };
```

**Step 2: Add credential check function before the server.listen block**

Before `server.listen(PORT, ...)` add:

```javascript
// --- Auth: check credentials and start auth flow if needed ---
function checkCredentials() {
  try {
    const raw = fs.readFileSync(CREDS_PATH, "utf-8");
    const creds = JSON.parse(raw);
    if (creds.claudeAiOauth?.accessToken || creds.oauthAccount?.accessToken) {
      return true;
    }
  } catch {
    // File missing or malformed
  }
  return false;
}
```

**Step 3: Commit**

```
git add scripts/instance/agent/server.js
git commit -m "feat(agent): add auth state constants and credential checker"
```

---

### Task 2: Agent — Background auth tab and URL polling

**Files:**
- Modify: `scripts/instance/agent/server.js` (add startAuthFlow and pollAuthUrl functions before server.listen)

**Step 1: Add the auth flow starter and URL poller**

After the `checkCredentials` function, add:

```javascript
let authPollTimer = null;

function startAuthFlow() {
  authState = { status: "checking", url: null };

  // Wait for Zellij session to be ready, then create auth tab
  function tryCreateAuthTab() {
    zellijAction(["query-tab-names"], (err) => {
      if (err) {
        // Zellij not ready yet, retry
        setTimeout(tryCreateAuthTab, 2000);
        return;
      }

      // Create the _auth tab and run claude
      zellijAction(
        ["new-tab", "--name", AUTH_TAB, "--cwd", DATA_ROOT],
        (err) => {
          if (err) {
            console.error("auth: failed to create auth tab:", err.message);
            setTimeout(tryCreateAuthTab, 3000);
            return;
          }

          // Type 'claude' to trigger the auth flow
          setTimeout(() => {
            zellijAction(["write-chars", "claude\n"], () => {});
          }, 500);

          console.log("auth: created _auth tab, starting URL poll");
          // Start polling for the auth URL
          authPollTimer = setInterval(pollAuthUrl, 2000);
        }
      );
    });
  }

  tryCreateAuthTab();
}

function pollAuthUrl() {
  // First check if credentials appeared (auth completed)
  if (checkCredentials()) {
    console.log("auth: credentials detected, authentication complete");
    authState = { status: "authenticated", url: null };
    clearInterval(authPollTimer);
    authPollTimer = null;
    // Close the _auth tab
    zellijAction(["go-to-tab-name", AUTH_TAB], () => {
      zellijAction(["close-tab"], () => {
        // Switch back to shell tab
        zellijAction(["go-to-tab-name", "shell"], () => {});
      });
    });
    return;
  }

  // Dump the auth tab screen to look for URLs
  zellijAction(["go-to-tab-name", AUTH_TAB], (err) => {
    if (err) return; // Tab may not exist yet
    const dumpPath = "/tmp/auth-dump-" + process.pid + ".txt";
    zellijAction(["dump-screen", dumpPath, "--full"], (err) => {
      // Switch back to shell tab so user doesn't see _auth if they connect
      zellijAction(["go-to-tab-name", "shell"], () => {});

      if (err) return;
      fs.readFile(dumpPath, "utf-8", (err, content) => {
        fs.unlink(dumpPath, () => {}); // cleanup
        if (err || !content) return;

        // Look for https:// URLs in the screen dump
        const urlRegex = /https:\/\/[^\s\x00-\x1f\]\)>"']+/g;
        const matches = content.match(urlRegex);
        if (matches && matches.length > 0) {
          // Pick the longest URL (most likely the auth URL, not a short link)
          const authUrl = matches.reduce((a, b) => a.length >= b.length ? a : b);
          if (authState.url !== authUrl) {
            console.log("auth: found URL:", authUrl);
          }
          authState = { status: "awaiting_auth", url: authUrl };
        }
      });
    });
  });
}
```

**Step 2: Add the startup hook inside the server.listen callback**

Change the `server.listen` block from:

```javascript
server.listen(PORT, "0.0.0.0", () => {
  console.log(`Instance agent listening on port ${PORT}`);
});
```

To:

```javascript
server.listen(PORT, "0.0.0.0", () => {
  console.log(`Instance agent listening on port ${PORT}`);

  // Check auth on startup
  if (checkCredentials()) {
    console.log("auth: credentials found, skipping auth flow");
    authState = { status: "authenticated", url: null };
  } else {
    console.log("auth: no credentials, starting auth flow");
    startAuthFlow();
  }
});
```

**Step 3: Commit**

```
git add scripts/instance/agent/server.js
git commit -m "feat(agent): background auth tab with URL polling"
```

---

### Task 3: Agent — Auth status endpoint and session filter

**Files:**
- Modify: `scripts/instance/agent/server.js` (add GET /auth/status endpoint, update /sessions filter)

**Step 1: Add the auth status endpoint**

After the `DELETE /tabs/:name` route block (after line 291), add:

```javascript
// --- GET /auth/status — Return current auth state ---
app.get("/auth/status", (req, res) => {
  res.json(authState);
});
```

**Step 2: Update /sessions to filter out internal tabs**

In the `/sessions` handler, change the filter line from:

```javascript
const projectTabs = tabNames.filter((t) => t !== "shell");
```

To:

```javascript
const projectTabs = tabNames.filter((t) => t !== "shell" && !t.startsWith("_"));
```

**Step 3: Update auto-cleanup to also filter internal tabs**

In the `cleanupIdleTabs` function, change:

```javascript
const projectTabs = tabNames.filter((t) => t !== "shell");
```

To:

```javascript
const projectTabs = tabNames.filter((t) => t !== "shell" && !t.startsWith("_"));
```

**Step 4: Commit**

```
git add scripts/instance/agent/server.js
git commit -m "feat(agent): add GET /auth/status endpoint, filter internal tabs from sessions"
```

---

### Task 4: Go proxy — Auth status route

**Files:**
- Modify: `internal/api/handler/proxy.go:233-237` (add AuthStatus method)
- Modify: `internal/api/server.go:110` (add route)

**Step 1: Add proxy method to proxy.go**

After the `DeleteTab` method (after line 237), add:

```go
// AuthStatus proxies GET /instances/{id}/auth/status to the agent.
func (h *ProxyHandler) AuthStatus(w http.ResponseWriter, r *http.Request) {
	h.proxyHTTP(w, r, "/auth/status")
}
```

**Step 2: Add route to server.go**

After the `r.Delete("/{id}/tabs/{name}", proxyH.DeleteTab)` line (line 110), add:

```go
			r.Get("/{id}/auth/status", proxyH.AuthStatus)
```

**Step 3: Commit**

```
git add internal/api/handler/proxy.go internal/api/server.go
git commit -m "feat(proxy): add auth status route"
```

---

### Task 5: Frontend — API method and type

**Files:**
- Modify: `web/lib/api.ts:63-68` (add AuthStatus interface)
- Modify: `web/lib/api.ts:219-228` (add getAuthStatus method)

**Step 1: Add the AuthStatus interface**

After the `SessionInfo` interface (after line 68), add:

```typescript
export interface AuthStatus {
  status: "authenticated" | "awaiting_auth" | "checking";
  url: string | null;
}
```

**Step 2: Add the API method**

After the `getSessionConversations` method (after line 228), add:

```typescript
  getAuthStatus(instanceId: number) {
    return apiFetch<AuthStatus>(`/instances/${instanceId}/auth/status`);
  },
```

**Step 3: Commit**

```
git add web/lib/api.ts
git commit -m "feat(frontend): add auth status type and API method"
```

---

### Task 6: Frontend — Terminal page auth gate

**Files:**
- Modify: `web/app/dashboard/terminal/page.tsx` (add auth check before showing terminal)

**Step 1: Rewrite TerminalContent to gate on auth**

Replace the entire `TerminalContent` function with:

```typescript
function TerminalContent() {
  const searchParams = useSearchParams();
  const tab = searchParams.get("tab");
  const [instance, setInstance] = useState<Instance | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [authStatus, setAuthStatus] = useState<string>("checking");
  const [authUrl, setAuthUrl] = useState<string | null>(null);

  useEffect(() => {
    api
      .getMyInstance()
      .then((inst) => {
        setInstance(inst);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load instance");
      })
      .finally(() => setLoading(false));
  }, []);

  // Poll auth status
  useEffect(() => {
    if (!instance || instance.status !== "running") return;

    let cancelled = false;
    function poll() {
      if (cancelled) return;
      api.getAuthStatus(instance!.id).then((auth) => {
        if (cancelled) return;
        setAuthStatus(auth.status);
        setAuthUrl(auth.url);
      }).catch(() => {});
    }

    poll();
    const interval = setInterval(poll, 3000);
    return () => { cancelled = true; clearInterval(interval); };
  }, [instance]);

  // Switch tab after auth is confirmed
  useEffect(() => {
    if (authStatus === "authenticated" && instance && tab) {
      api.createTab(instance.id, tab, tab).catch(() => {});
    }
  }, [authStatus, instance, tab]);

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

  // Auth gate
  if (authStatus !== "authenticated") {
    return (
      <div className="flex h-[calc(100vh-8rem)] items-center justify-center">
        <div className="mx-auto max-w-md text-center">
          <div className="rounded-lg bg-white p-8 ring-1 ring-gray-200">
            <h2 className="text-lg font-semibold text-gray-900">
              Sign in to Claude Code
            </h2>
            <p className="mt-2 text-sm text-gray-600">
              Your instance needs to authenticate with Claude before you can use
              the terminal.
            </p>
            {authUrl ? (
              <a
                href={authUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-4 inline-block rounded bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
              >
                Sign into Claude
              </a>
            ) : (
              <div className="mt-4 flex items-center justify-center gap-2 text-sm text-gray-500">
                <svg
                  className="h-4 w-4 animate-spin"
                  viewBox="0 0 24 24"
                  fill="none"
                >
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                  />
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
                  />
                </svg>
                Preparing authentication...
              </div>
            )}
            {authStatus === "awaiting_auth" && authUrl && (
              <p className="mt-3 text-xs text-gray-400">
                Waiting for you to complete sign-in...
              </p>
            )}
          </div>
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

Also update the import to include `AuthStatus`:

```typescript
import { api, Instance, AuthStatus } from "@/lib/api";
```

**Step 2: Commit**

```
git add web/app/dashboard/terminal/page.tsx
git commit -m "feat(terminal): gate terminal behind auth status with sign-in card"
```

---

### Task 7: Frontend — Projects page auth banner and button disable

**Files:**
- Modify: `web/app/dashboard/projects/page.tsx` (add auth status polling, banner, pass to ProjectCard)
- Modify: `web/components/ProjectCard.tsx` (accept and respect `authenticated` prop)

**Step 1: Update ProjectsPage to track auth and pass to cards**

In `web/app/dashboard/projects/page.tsx`:

Add `AuthStatus` to the import:

```typescript
import { api, Instance, Project, SessionInfo, AuthStatus } from "@/lib/api";
```

Add auth state after the existing `useState` hooks:

```typescript
const [authStatus, setAuthStatus] = useState<AuthStatus>({ status: "checking", url: null });
```

Add auth polling inside the existing `useEffect` where `loadSessions` is called. After `pollRef.current = setInterval(...)`:

```typescript
          // Also poll auth status
          const pollAuth = () => api.getAuthStatus(inst.id).then(setAuthStatus).catch(() => {});
          pollAuth();
          const authInterval = setInterval(pollAuth, 3000);
          // Store for cleanup
          (pollRef as any)._authInterval = authInterval;
```

Update the cleanup to also clear the auth interval:

```typescript
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
      if ((pollRef as any)._authInterval) clearInterval((pollRef as any)._authInterval);
    };
```

Add the auth banner before the projects list, after `<CloneRepoForm>`:

```tsx
      {authStatus.status !== "authenticated" && (
        <div className="rounded-lg bg-amber-50 p-4 ring-1 ring-amber-200">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-amber-800">
                Claude Code needs authentication
              </p>
              <p className="mt-1 text-xs text-amber-600">
                Sign in to start using your projects.
              </p>
            </div>
            {authStatus.url ? (
              <a
                href={authStatus.url}
                target="_blank"
                rel="noopener noreferrer"
                className="rounded bg-amber-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-amber-700"
              >
                Sign into Claude
              </a>
            ) : (
              <span className="text-xs text-amber-500">Preparing...</span>
            )}
          </div>
        </div>
      )}
```

Pass `authenticated` to each ProjectCard:

```tsx
              <ProjectCard
                key={project.path}
                project={project}
                instance={instance}
                session={getSessionForProject(project)}
                authenticated={authStatus.status === "authenticated"}
              />
```

**Step 2: Update ProjectCard to accept `authenticated` prop**

In `web/components/ProjectCard.tsx`, add to the interface:

```typescript
interface ProjectCardProps {
  project: Project;
  instance: Instance;
  session?: SessionInfo;
  authenticated?: boolean;
}
```

Update the destructure:

```typescript
export default function ProjectCard({ project, instance, session, authenticated = true }: ProjectCardProps) {
```

All action buttons already use `disabled={acting}`. Add auth gating by changing each `disabled={acting}` to `disabled={acting || !authenticated}`.

There are 6 button occurrences to update (New Chat x2, Open Terminal, Stop, Continue, plus the confirm Confirm button — leave Cancel alone).

**Step 3: Commit**

```
git add web/app/dashboard/projects/page.tsx web/components/ProjectCard.tsx
git commit -m "feat(projects): auth banner and disable actions until authenticated"
```

---

### Task 8: Build verification and manual test

**Step 1: Build the Go backend**

Run: `cd /home/logan/scratch/cloudcode && go build ./...`
Expected: No compile errors

**Step 2: Build the frontend**

Run: `cd /home/logan/scratch/cloudcode/web && npx next build`
Expected: No type errors, build succeeds

**Step 3: Fix any issues found**

If either build fails, fix the errors and re-run.

**Step 4: Commit any fixes**

```
git add -A
git commit -m "fix: resolve build errors from auth URL capture feature"
```

---

### Task 9: Final commit and push

**Step 1: Push all changes**

```
git push
```

User should then rebuild the instance (`./scripts/bootstrap.sh --rebuild --no-e2e`) and test the flow:
1. Fresh instance with no credentials → should see "Sign into Claude" card
2. Click the button → opens auth URL in new browser tab
3. Complete auth → terminal unlocks automatically
