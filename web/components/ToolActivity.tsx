"use client";

import { useState } from "react";

export interface ToolEvent {
  type: "tool_use" | "tool_result";
  tool: string;
  input?: unknown;
  output?: string;
}

interface ToolActivityProps {
  events: ToolEvent[];
}

export default function ToolActivity({ events }: ToolActivityProps) {
  const [expanded, setExpanded] = useState(false);

  if (events.length === 0) return null;

  const toolNames = Array.from(new Set(events.filter((e) => e.type === "tool_use").map((e) => e.tool)));
  const summary = toolNames.length > 0
    ? `Used ${toolNames.join(", ")}`
    : `${events.length} tool event(s)`;

  return (
    <div className="my-1 rounded border border-gray-200 bg-gray-50 text-xs">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-gray-600 hover:bg-gray-100"
      >
        <span className={`transition-transform ${expanded ? "rotate-90" : ""}`}>
          &#9654;
        </span>
        <span className="font-medium">{summary}</span>
      </button>
      {expanded && (
        <div className="space-y-1 border-t border-gray-200 px-3 py-2">
          {events.map((event, i) => (
            <div key={i} className="font-mono">
              {event.type === "tool_use" ? (
                <div>
                  <span className="text-blue-600">{event.tool}</span>
                  {event.input != null && (
                    <span className="ml-2 text-gray-500">
                      {typeof event.input === "string"
                        ? (event.input as string).slice(0, 100)
                        : JSON.stringify(event.input).slice(0, 100)}
                    </span>
                  )}
                </div>
              ) : (
                <div className="max-h-24 overflow-y-auto whitespace-pre-wrap text-gray-700">
                  {event.output?.slice(0, 500)}
                  {(event.output?.length || 0) > 500 ? "..." : ""}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
