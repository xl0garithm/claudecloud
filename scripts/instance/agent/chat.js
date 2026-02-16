/**
 * Claude Code SDK wrapper for instance agent.
 * Provides a streaming chat session backed by @anthropic-ai/claude-code.
 */

const { query } = require("@anthropic-ai/claude-code");

/**
 * Extracts plain text from a Claude SDK message or result value.
 * Handles multiple formats:
 *   - string: returned as-is
 *   - {content: [{type:"text", text:"..."}]} (Anthropic API content blocks)
 *   - {content: "string"}
 *   - {text: "string"}
 */
function extractText(value) {
  if (typeof value === "string") return value;
  if (!value) return "";

  // Handle content block arrays: [{type:"text", text:"..."}, ...]
  const content = value.content;
  if (Array.isArray(content)) {
    return content
      .filter((block) => block.type === "text")
      .map((block) => block.text || "")
      .join("");
  }
  if (typeof content === "string") return content;

  // Direct text property
  if (typeof value.text === "string") return value.text;

  return "";
}

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
        // Assistant events may contain content blocks (text + tool_use).
        // Extract text blocks and emit separately from tool calls.
        const message = event.message;
        const blocks = message?.content;

        if (Array.isArray(blocks)) {
          // Emit text from text blocks
          const text = blocks
            .filter((b) => b.type === "text")
            .map((b) => b.text || "")
            .join("");
          if (text) {
            yield { type: "text", content: text };
          }

          // Emit tool_use blocks as separate events
          for (const block of blocks) {
            if (block.type === "tool_use") {
              yield {
                type: "tool_use",
                tool: block.name,
                input: block.input,
              };
            }
          }
        } else {
          // Fallback: extract text however we can
          const text = extractText(message);
          if (text) {
            yield { type: "text", content: text };
          }
        }
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
          content: extractText(event.result),
        };
      }
    }
  } catch (err) {
    if (err.name === "AbortError") return;
    yield { type: "error", content: err.message };
  }
}

module.exports = { createSession };
