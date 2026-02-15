"use client";

import { useEffect, useState, useCallback } from "react";
import { api, Instance, Project } from "@/lib/api";
import ProjectCard from "@/components/ProjectCard";
import CloneRepoForm from "@/components/CloneRepoForm";

export default function ProjectsPage() {
  const [instance, setInstance] = useState<Instance | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const loadProjects = useCallback(async (inst: Instance) => {
    try {
      const projs = await api.getProjects(inst.id);
      setProjects(projs);
    } catch {
      // Instance may not be running — ignore
    }
  }, []);

  useEffect(() => {
    api
      .getMyInstance()
      .then((inst) => {
        setInstance(inst);
        if (inst.status === "running") {
          loadProjects(inst);
        }
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load instance");
      })
      .finally(() => setLoading(false));
  }, [loadProjects]);

  async function handleClone(url: string, branch?: string) {
    if (!instance) return;
    await api.cloneProject(instance.id, url, branch);
    await loadProjects(instance);
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
            Instance is {instance.status}. Wake it from the Overview tab to
            manage projects.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <CloneRepoForm onClone={handleClone} />

      <div>
        <h2 className="mb-4 text-lg font-semibold">Projects</h2>
        {projects.length === 0 ? (
          <div className="rounded-lg bg-white p-8 text-center ring-1 ring-gray-200">
            <p className="text-gray-600">No projects yet.</p>
            <p className="mt-1 text-sm text-gray-500">
              Clone a repository above to get started.
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {projects.map((project) => (
              <ProjectCard key={project.path} project={project} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
