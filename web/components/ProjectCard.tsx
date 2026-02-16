"use client";

import { useRouter } from "next/navigation";
import Link from "next/link";
import { useState } from "react";
import { api, Project, Instance } from "@/lib/api";

interface ProjectCardProps {
  project: Project;
  instance: Instance;
}

export default function ProjectCard({ project, instance }: ProjectCardProps) {
  const router = useRouter();
  const [opening, setOpening] = useState(false);

  async function handleOpenTerminal() {
    setOpening(true);
    try {
      // Create a Zellij tab for this project (or reuse existing)
      await api.createTab(instance.id, project.name, project.path);
      router.push("/dashboard/terminal");
    } catch {
      // Navigate anyway — tab creation is best-effort
      router.push("/dashboard/terminal");
    } finally {
      setOpening(false);
    }
  }

  return (
    <div className="rounded-lg bg-white p-4 ring-1 ring-gray-200">
      <div className="flex items-start justify-between">
        <div>
          <h3 className="font-semibold text-gray-900">{project.name}</h3>
          <p className="mt-1 text-xs text-gray-500">/{project.path}</p>
          {project.remoteUrl && (
            <p className="mt-1 text-xs text-gray-400 truncate max-w-sm">
              {project.remoteUrl}
            </p>
          )}
        </div>
        <div className="flex gap-2">
          <Link
            href={`/dashboard/chat?cwd=${encodeURIComponent(project.path)}`}
            className="rounded bg-gray-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-gray-800"
          >
            Open in Chat
          </Link>
          <button
            onClick={handleOpenTerminal}
            disabled={opening}
            className="rounded bg-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-300 disabled:opacity-50"
          >
            {opening ? "Opening..." : "Open in Terminal"}
          </button>
        </div>
      </div>
    </div>
  );
}
