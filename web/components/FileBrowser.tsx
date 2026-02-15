"use client";

import { useEffect, useState, useCallback } from "react";
import { api, FileEntry } from "@/lib/api";

interface FileBrowserProps {
  instanceId: number;
  currentPath: string;
  onNavigate: (path: string) => void;
  onFileSelect?: (path: string) => void;
}

export default function FileBrowser({
  instanceId,
  currentPath,
  onNavigate,
  onFileSelect,
}: FileBrowserProps) {
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [preview, setPreview] = useState<{
    path: string;
    content: string;
  } | null>(null);
  const [loading, setLoading] = useState(false);

  const loadDir = useCallback(
    async (dirPath: string) => {
      setLoading(true);
      setPreview(null);
      try {
        const files = await api.getFiles(instanceId, dirPath || undefined);
        setEntries(files);
      } catch {
        setEntries([]);
      } finally {
        setLoading(false);
      }
    },
    [instanceId]
  );

  useEffect(() => {
    loadDir(currentPath);
  }, [currentPath, loadDir]);

  async function handleClick(entry: FileEntry) {
    if (entry.type === "directory") {
      onNavigate(entry.path);
    } else {
      if (onFileSelect) {
        onFileSelect(entry.path);
      }
      try {
        const result = await api.readFile(instanceId, entry.path);
        setPreview({ path: entry.path, content: result.content });
      } catch {
        setPreview({ path: entry.path, content: "(failed to read file)" });
      }
    }
  }

  const parentPath = currentPath
    ? currentPath.split("/").slice(0, -1).join("/")
    : "";

  return (
    <div className="flex h-full flex-col overflow-hidden border-r border-gray-200 bg-gray-50">
      {/* Path breadcrumb */}
      <div className="border-b border-gray-200 px-3 py-2 text-xs font-medium text-gray-600">
        /{currentPath || "claude-data"}
      </div>

      {/* File list */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="p-3 text-xs text-gray-500">Loading...</div>
        ) : (
          <ul className="text-sm">
            {currentPath && (
              <li>
                <button
                  onClick={() => onNavigate(parentPath)}
                  className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-gray-600 hover:bg-gray-100"
                >
                  <span className="text-xs">&#128193;</span>
                  <span>..</span>
                </button>
              </li>
            )}
            {entries.map((entry) => (
              <li key={entry.path}>
                <button
                  onClick={() => handleClick(entry)}
                  className="flex w-full items-center gap-2 px-3 py-1.5 text-left hover:bg-gray-100"
                >
                  <span className="text-xs">
                    {entry.type === "directory" ? "\uD83D\uDCC1" : "\uD83D\uDCC4"}
                  </span>
                  <span
                    className={
                      entry.type === "directory"
                        ? "font-medium text-gray-800"
                        : "text-gray-600"
                    }
                  >
                    {entry.name}
                  </span>
                </button>
              </li>
            ))}
            {entries.length === 0 && !loading && (
              <li className="px-3 py-2 text-xs text-gray-500">Empty directory</li>
            )}
          </ul>
        )}
      </div>

      {/* File preview */}
      {preview && (
        <div className="border-t border-gray-200">
          <div className="flex items-center justify-between bg-gray-100 px-3 py-1.5 text-xs">
            <span className="font-medium text-gray-700">{preview.path}</span>
            <button
              onClick={() => setPreview(null)}
              className="text-gray-500 hover:text-gray-700"
            >
              Close
            </button>
          </div>
          <pre className="max-h-48 overflow-auto bg-white p-3 text-xs text-gray-700">
            {preview.content.slice(0, 5000)}
            {preview.content.length > 5000 ? "\n...(truncated)" : ""}
          </pre>
        </div>
      )}
    </div>
  );
}
