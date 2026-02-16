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
        await api.createTab(instance.id, project.name, project.path, { resume: true });
        router.push(`/dashboard/terminal?tab=${encodeURIComponent(project.name)}`);
      } else {
        setConversations(convs);
        setShowConversations(true);
      }
    } catch {
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
