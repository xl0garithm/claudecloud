"use client";

import Link from "next/link";
import { Project } from "@/lib/api";

interface ProjectCardProps {
  project: Project;
}

export default function ProjectCard({ project }: ProjectCardProps) {
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
          <Link
            href="/dashboard/terminal"
            className="rounded bg-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-300"
          >
            Open in Terminal
          </Link>
        </div>
      </div>
    </div>
  );
}
