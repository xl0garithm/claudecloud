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

// --- POST /tabs — Create a Zellij tab for a project ---
app.post("/tabs", (req, res) => {
  const { name, cwd, command } = req.body;
  if (!name || typeof name !== "string") {
    return res.status(400).json({ error: "name is required" });
  }

  const tabCwd = safePath(cwd || "") || DATA_ROOT;
  const tabCommand = command || "claude";
  const session = "main";

  // Check if a tab with this name already exists by querying Zellij
  execFile(
    "zellij",
    ["action", "query-tab-names"],
    { env: { ...process.env, ZELLIJ_SESSION_NAME: session }, timeout: 5000 },
    (err, stdout) => {
      if (!err && stdout.includes(name)) {
        // Tab already exists — switch to it and report success
        execFile(
          "zellij",
          ["action", "go-to-tab-name", name],
          { env: { ...process.env, ZELLIJ_SESSION_NAME: session }, timeout: 5000 },
          () => {} // best-effort
        );
        return res.json({ status: "exists", tab: name });
      }

      // Create a new tab
      execFile(
        "zellij",
        ["action", "new-tab", "--name", name, "--cwd", tabCwd],
        { env: { ...process.env, ZELLIJ_SESSION_NAME: session }, timeout: 5000 },
        (err) => {
          if (err) {
            return res.status(500).json({ error: "failed to create tab: " + err.message });
          }

          // Start the command in the new tab
          if (tabCommand) {
            execFile(
              "zellij",
              ["action", "write-chars", tabCommand + "\n"],
              { env: { ...process.env, ZELLIJ_SESSION_NAME: session }, timeout: 5000 },
              () => {
                // Best-effort — don't fail if write-chars has issues
              }
            );
          }

          res.json({ status: "created", tab: name });
        }
      );
    }
  );
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

server.listen(PORT, "0.0.0.0", () => {
  console.log(`Instance agent listening on port ${PORT}`);
});
