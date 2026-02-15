/**
 * Claude Code SDK wrapper for instance agent.
 * Provides a streaming chat session backed by @anthropic-ai/claude-code.
 */

const { query } = require("@anthropic-ai/claude-code");

/**
 * Creates a streaming chat session.
 * @param {string} userMessage - The user's message
 * @param {string} cwd - Working directory for the session
 * @param {AbortSignal} [signal] - Optional abort signal
 * @returns {AsyncIterable} Stream of response events
 */
async function* createSession(userMessage, cwd, signal) {
  try {
    const response = await query({
      prompt: userMessage,
      options: {
        cwd: cwd || "/claude-data",
        allowedTools: ["Read", "Write", "Edit", "Bash", "Glob", "Grep"],
      },
      signal,
    });

    for await (const event of response) {
      if (event.type === "assistant") {
        yield {
          type: "text",
          content:
            typeof event.message === "string"
              ? event.message
              : event.message?.content || "",
        };
      } else if (event.type === "tool_use") {
        yield {
          type: "tool_use",
          tool: event.tool || event.name,
          input: event.input,
        };
      } else if (event.type === "tool_result") {
        yield {
          type: "tool_result",
          tool: event.tool || event.name,
          output:
            typeof event.output === "string"
              ? event.output
              : JSON.stringify(event.output),
        };
      } else if (event.type === "result") {
        yield {
          type: "done",
          content:
            typeof event.result === "string"
              ? event.result
              : event.result?.content || "",
        };
      }
    }
  } catch (err) {
    if (err.name === "AbortError") return;
    yield { type: "error", content: err.message };
  }
}

module.exports = { createSession };
