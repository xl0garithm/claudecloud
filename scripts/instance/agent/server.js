/**
 * Instance Agent Server
 *
 * Express + WebSocket server running on each instance (port 3001).
 * Provides:
 *   WS  /chat          — Claude Code SDK streaming chat
 *   GET /files          — Directory listing
 *   GET /files/read     — File content
 *   GET /projects       — Scan for .git directories
 *   POST /projects/clone — Git clone a repository
 *
 * Auth: AGENT_SECRET env var checked via Authorization header.
 * All file ops restricted to /claude-data/.
 */

const express = require("express");
const http = require("http");
const { WebSocketServer } = require("ws");
const fs = require("fs");
const path = require("path");
const { execFile } = require("child_process");
const { createSession } = require("./chat");

const PORT = process.env.AGENT_PORT || 3001;
const AGENT_SECRET = process.env.AGENT_SECRET || "";
const DATA_ROOT = "/claude-data";

const app = express();
app.use(express.json());

// --- Auth middleware ---
function authMiddleware(req, res, next) {
  const authHeader = req.headers.authorization || "";
  const token = authHeader.replace(/^Bearer\s+/i, "");
  if (!AGENT_SECRET || token !== AGENT_SECRET) {
    return res.status(401).json({ error: "unauthorized" });
  }
  next();
}

app.use(authMiddleware);

// --- Path validation ---
function safePath(userPath) {
  const resolved = path.resolve(DATA_ROOT, userPath || "");
  if (!resolved.startsWith(DATA_ROOT)) {
    return null;
  }
  return resolved;
}

// --- GET /files?path=... ---
app.get("/files", (req, res) => {
  const dirPath = safePath(req.query.path || "");
  if (!dirPath) {
    return res.status(400).json({ error: "invalid path" });
  }

  fs.readdir(dirPath, { withFileTypes: true }, (err, entries) => {
    if (err) {
      if (err.code === "ENOENT") return res.status(404).json({ error: "not found" });
      return res.status(500).json({ error: err.message });
    }
    const result = entries.map((e) => ({
      name: e.name,
      type: e.isDirectory() ? "directory" : "file",
      path: path.relative(DATA_ROOT, path.join(dirPath, e.name)),
    }));
    res.json(result);
  });
});

// --- GET /files/read?path=... ---
app.get("/files/read", (req, res) => {
  const filePath = safePath(req.query.path || "");
  if (!filePath) {
    return res.status(400).json({ error: "invalid path" });
  }

  fs.readFile(filePath, "utf-8", (err, content) => {
    if (err) {
      if (err.code === "ENOENT") return res.status(404).json({ error: "not found" });
      return res.status(500).json({ error: err.message });
    }
    // Limit response to 1MB
    const truncated = content.length > 1048576;
    res.json({
      path: req.query.path,
      content: truncated ? content.slice(0, 1048576) : content,
      truncated,
    });
  });
});

// --- GET /projects ---
app.get("/projects", (req, res) => {
  const projects = [];

  function scanDir(dir, depth) {
    if (depth > 3) return;
    let entries;
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      if (entry.name.startsWith(".") && entry.name !== ".git") continue;
      const fullPath = path.join(dir, entry.name);
      if (entry.name === ".git") {
        const projectPath = path.dirname(fullPath);
        const relativePath = path.relative(DATA_ROOT, projectPath);
        // Try to read git remote
        let remoteUrl = "";
        try {
          const configPath = path.join(fullPath, "config");
          const gitConfig = fs.readFileSync(configPath, "utf-8");
          const match = gitConfig.match(/url\s*=\s*(.+)/);
          if (match) remoteUrl = match[1].trim();
        } catch {
          // ignore
        }
        projects.push({
          name: path.basename(projectPath),
          path: relativePath,
          remoteUrl,
        });
      } else {
        scanDir(fullPath, depth + 1);
      }
    }
  }

  scanDir(DATA_ROOT, 0);
  res.json(projects);
});

// --- POST /projects/clone ---
app.post("/projects/clone", (req, res) => {
  const { url, branch } = req.body;
  if (!url || typeof url !== "string") {
    return res.status(400).json({ error: "url is required" });
  }

  // Validate URL looks like a git URL
  if (!url.match(/^(https?:\/\/|git@|ssh:\/\/)/)) {
    return res.status(400).json({ error: "invalid git URL" });
  }

  const args = ["clone"];
  if (branch) args.push("-b", branch);
  args.push(url);

  execFile("git", args, { cwd: DATA_ROOT, timeout: 120000 }, (err, stdout, stderr) => {
    if (err) {
      return res.status(500).json({ error: stderr || err.message });
    }
    res.json({ status: "cloned", output: stdout || stderr });
  });
});

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
            // Write the claude command into the pane if resuming
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
      const projectTabs = tabNames.filter((t) => t !== "shell");

      if (projectTabs.length === 0) {
        return res.json([]);
      }

      const sessions = [];
      let pending = projectTabs.length;

      projectTabs.forEach((tabName) => {
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

// --- GET /sessions/:project/conversations — List recent conversations for resume ---
app.get("/sessions/:project/conversations", (req, res) => {
  const { project } = req.params;
  const projectDir = safePath(project);
  if (!projectDir) {
    return res.status(400).json({ error: "invalid project" });
  }

  // Scan ~/.claude/projects/ for conversation data
  const claudeProjectsDir = path.join("/home/claude/.claude/projects");

  fs.readdir(claudeProjectsDir, { withFileTypes: true }, (err, entries) => {
    if (err) {
      return res.json([]); // No conversations yet
    }

    const conversations = [];
    const encodedPath = projectDir.replace(/\//g, "-");

    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      if (entry.name.includes(project) || entry.name.includes(encodedPath)) {
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

    res.json(conversations.slice(0, 20));
  });
});

// --- HTTP + WebSocket server ---
const server = http.createServer(app);
const wss = new WebSocketServer({ server, path: "/chat" });

wss.on("connection", (ws, req) => {
  // Auth check for WebSocket
  const authHeader = req.headers.authorization || "";
  const urlParams = new URL(req.url, `http://localhost:${PORT}`).searchParams;
  const token =
    authHeader.replace(/^Bearer\s+/i, "") || urlParams.get("secret") || "";

  if (!AGENT_SECRET || token !== AGENT_SECRET) {
    ws.close(4001, "unauthorized");
    return;
  }

  let abortController = null;

  ws.on("message", async (data) => {
    let msg;
    try {
      msg = JSON.parse(data.toString());
    } catch {
      ws.send(JSON.stringify({ type: "error", content: "invalid JSON" }));
      return;
    }

    if (msg.type === "abort" && abortController) {
      abortController.abort();
      return;
    }

    if (msg.type !== "message" || !msg.content) {
      ws.send(JSON.stringify({ type: "error", content: "expected {type:'message', content:'...'}" }));
      return;
    }

    // Cancel any in-flight request
    if (abortController) abortController.abort();
    abortController = new AbortController();

    const cwd = safePath(msg.cwd || "") || DATA_ROOT;

    try {
      for await (const event of createSession(msg.content, cwd, abortController.signal)) {
        if (ws.readyState !== ws.OPEN) break;
        ws.send(JSON.stringify(event));
      }
    } catch (err) {
      if (ws.readyState === ws.OPEN) {
        ws.send(JSON.stringify({ type: "error", content: err.message }));
      }
    }
    abortController = null;
  });

  ws.on("close", () => {
    if (abortController) abortController.abort();
  });
});

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
        execFile("pgrep", ["-f", `claude.*${tabName}`], { timeout: 3000 }, (err) => {
          if (!err) {
            // Claude is running — not idle
            idleSince.delete(tabName);
            return;
          }

          if (!idleSince.has(tabName)) {
            idleSince.set(tabName, now);
            return;
          }

          const idleDuration = now - idleSince.get(tabName);
          if (idleDuration >= IDLE_TIMEOUT_MS) {
            console.log(`auto-cleanup: closing idle tab "${tabName}" (idle ${Math.round(idleDuration / 60000)}m)`);
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

server.listen(PORT, "0.0.0.0", () => {
  console.log(`Instance agent listening on port ${PORT}`);
});
