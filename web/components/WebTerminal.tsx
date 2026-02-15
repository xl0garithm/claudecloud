"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import "@xterm/xterm/css/xterm.css";
import { createAuthWsUrl } from "@/lib/ws";

interface WebTerminalProps {
  instanceId: number;
}

type ConnectionStatus = "connecting" | "connected" | "disconnected" | "error";

export default function WebTerminal({ instanceId }: WebTerminalProps) {
  const termRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const [status, setStatus] = useState<ConnectionStatus>("connecting");

  const connect = useCallback(() => {
    if (!termRef.current) return;

    // Clean up previous terminal
    if (terminalRef.current) {
      terminalRef.current.dispose();
    }
    if (wsRef.current) {
      wsRef.current.close();
    }

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
      theme: {
        background: "#1a1b26",
        foreground: "#c0caf5",
        cursor: "#c0caf5",
        selectionBackground: "#33467c",
      },
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.loadAddon(new WebLinksAddon());

    term.open(termRef.current);
    fitAddon.fit();

    terminalRef.current = term;
    fitAddonRef.current = fitAddon;

    setStatus("connecting");

    const wsUrl = createAuthWsUrl(`/instances/${instanceId}/terminal`);
    const ws = new WebSocket(wsUrl);
    ws.binaryType = "arraybuffer";
    wsRef.current = ws;

    ws.onopen = () => {
      setStatus("connected");
    };

    ws.onmessage = (event) => {
      const data = new Uint8Array(event.data as ArrayBuffer);
      if (data.length === 0) return;

      // ttyd protocol: byte 0 = message type
      // 0 = output data, 1 = set window title, 2 = set prefs
      const msgType = data[0];
      const payload = data.slice(1);

      switch (msgType) {
        case 0: // output
          term.write(payload);
          break;
        case 1: // title
          // ignored
          break;
        case 2: // prefs
          // ignored
          break;
      }
    };

    ws.onclose = () => {
      setStatus("disconnected");
    };

    ws.onerror = () => {
      setStatus("error");
    };

    // Input: ttyd expects byte 0=input, followed by data
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        const encoder = new TextEncoder();
        const payload = encoder.encode(data);
        const msg = new Uint8Array(payload.length + 1);
        msg[0] = 0; // input type
        msg.set(payload, 1);
        ws.send(msg.buffer);
      }
    });

    // Resize: ttyd expects byte 2=resize, followed by JSON {columns, rows}
    term.onResize(({ cols, rows }) => {
      if (ws.readyState === WebSocket.OPEN) {
        const resizeMsg = JSON.stringify({ columns: cols, rows: rows });
        const encoder = new TextEncoder();
        const payload = encoder.encode(resizeMsg);
        const msg = new Uint8Array(payload.length + 1);
        msg[0] = 1; // resize type
        msg.set(payload, 1);
        ws.send(msg.buffer);
      }
    });
  }, [instanceId]);

  useEffect(() => {
    connect();

    const handleResize = () => {
      if (fitAddonRef.current) {
        fitAddonRef.current.fit();
      }
    };
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      if (wsRef.current) wsRef.current.close();
      if (terminalRef.current) terminalRef.current.dispose();
    };
  }, [connect]);

  return (
    <div className="flex h-full flex-col">
      {/* Status bar */}
      <div className="flex items-center justify-between bg-gray-800 px-3 py-1.5 text-xs">
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2 w-2 rounded-full ${
              status === "connected"
                ? "bg-green-400"
                : status === "connecting"
                  ? "bg-yellow-400"
                  : "bg-red-400"
            }`}
          />
          <span className="text-gray-300">
            {status === "connected"
              ? "Connected"
              : status === "connecting"
                ? "Connecting..."
                : status === "disconnected"
                  ? "Disconnected"
                  : "Connection error"}
          </span>
        </div>
        {(status === "disconnected" || status === "error") && (
          <button
            onClick={connect}
            className="rounded bg-gray-700 px-2 py-0.5 text-gray-300 hover:bg-gray-600"
          >
            Reconnect
          </button>
        )}
      </div>

      {/* Terminal */}
      <div ref={termRef} className="flex-1 bg-[#1a1b26]" />
    </div>
  );
}
