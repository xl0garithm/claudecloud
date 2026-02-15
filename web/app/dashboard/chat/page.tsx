"use client";

import { Suspense, useEffect, useState, useRef, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import { api, Instance } from "@/lib/api";
import { createAuthWsUrl } from "@/lib/ws";
import ChatMessage, { Message } from "@/components/ChatMessage";
import ChatInput from "@/components/ChatInput";
import FileBrowser from "@/components/FileBrowser";
import { ToolEvent } from "@/components/ToolActivity";

export default function ChatPage() {
  return (
    <Suspense fallback={<div className="flex h-[calc(100vh-8rem)] items-center justify-center text-gray-500">Loading...</div>}>
      <ChatPageInner />
    </Suspense>
  );
}

function ChatPageInner() {
  const searchParams = useSearchParams();
  const initialCwd = searchParams.get("cwd") || "";

  const [instance, setInstance] = useState<Instance | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [cwd, setCwd] = useState(initialCwd);
  const [showFiles, setShowFiles] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const pendingToolEventsRef = useRef<ToolEvent[]>([]);
  const pendingTextRef = useRef("");

  useEffect(() => {
    api
      .getMyInstance()
      .then(setInstance)
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load instance");
      })
      .finally(() => setLoading(false));
  }, []);

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const connectWs = useCallback(
    (inst: Instance) => {
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) return;

      const wsUrl = createAuthWsUrl(`/instances/${inst.id}/chat`);
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onmessage = (event) => {
        let data;
        try {
          data = JSON.parse(event.data);
        } catch {
          return;
        }

        switch (data.type) {
          case "text":
            pendingTextRef.current += data.content;
            setMessages((prev) => {
              const last = prev[prev.length - 1];
              if (last && last.role === "assistant") {
                return [
                  ...prev.slice(0, -1),
                  { ...last, content: pendingTextRef.current },
                ];
              }
              return [
                ...prev,
                {
                  role: "assistant",
                  content: pendingTextRef.current,
                  toolEvents: [],
                },
              ];
            });
            break;

          case "tool_use":
          case "tool_result":
            pendingToolEventsRef.current.push(data as ToolEvent);
            setMessages((prev) => {
              const last = prev[prev.length - 1];
              if (last && last.role === "assistant") {
                return [
                  ...prev.slice(0, -1),
                  { ...last, toolEvents: [...pendingToolEventsRef.current] },
                ];
              }
              return prev;
            });
            break;

          case "done":
            if (data.content && data.content !== pendingTextRef.current) {
              pendingTextRef.current = data.content;
              setMessages((prev) => {
                const last = prev[prev.length - 1];
                if (last && last.role === "assistant") {
                  return [
                    ...prev.slice(0, -1),
                    { ...last, content: data.content },
                  ];
                }
                return prev;
              });
            }
            setStreaming(false);
            break;

          case "error":
            setMessages((prev) => [
              ...prev,
              {
                role: "assistant",
                content: `Error: ${data.content}`,
              },
            ]);
            setStreaming(false);
            break;
        }
      };

      ws.onclose = () => {
        setStreaming(false);
      };

      ws.onerror = () => {
        setStreaming(false);
      };
    },
    []
  );

  useEffect(() => {
    if (instance && instance.status === "running") {
      connectWs(instance);
    }
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [instance, connectWs]);

  function handleSend(content: string) {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      if (instance) connectWs(instance);
      // Wait briefly for connection
      setTimeout(() => handleSend(content), 500);
      return;
    }

    setMessages((prev) => [...prev, { role: "user", content }]);
    setStreaming(true);
    pendingTextRef.current = "";
    pendingToolEventsRef.current = [];

    wsRef.current.send(
      JSON.stringify({
        type: "message",
        content,
        cwd: cwd || undefined,
      })
    );
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
            Instance is {instance.status}. Wake it from the Overview tab to chat.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-[calc(100vh-8rem)] overflow-hidden rounded-lg ring-1 ring-gray-200">
      {/* File browser sidebar */}
      {showFiles && (
        <div className="w-64 flex-shrink-0">
          <FileBrowser
            instanceId={instance.id}
            currentPath={cwd}
            onNavigate={setCwd}
          />
        </div>
      )}

      {/* Chat area */}
      <div className="flex flex-1 flex-col">
        {/* Toolbar */}
        <div className="flex items-center gap-3 border-b border-gray-200 bg-white px-4 py-2 text-sm">
          <button
            onClick={() => setShowFiles(!showFiles)}
            className={`rounded px-2 py-1 ${
              showFiles
                ? "bg-gray-200 text-gray-900"
                : "text-gray-600 hover:bg-gray-100"
            }`}
          >
            Files
          </button>
          {cwd && (
            <span className="text-xs text-gray-500">
              cwd: /{cwd}
            </span>
          )}
          {streaming && (
            <span className="ml-auto text-xs text-gray-400">Streaming...</span>
          )}
        </div>

        {/* Messages */}
        <div className="flex-1 space-y-4 overflow-y-auto bg-gray-50 p-4">
          {messages.length === 0 && (
            <div className="flex h-full items-center justify-center text-gray-400">
              Send a message to start chatting with Claude
            </div>
          )}
          {messages.map((msg, i) => (
            <ChatMessage key={i} message={msg} />
          ))}
          <div ref={messagesEndRef} />
        </div>

        {/* Input */}
        <ChatInput onSend={handleSend} disabled={streaming} />
      </div>
    </div>
  );
}
