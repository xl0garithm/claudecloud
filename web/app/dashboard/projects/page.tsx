"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api, Instance, Project, SessionInfo, AuthStatus } from "@/lib/api";
import ProjectCard from "@/components/ProjectCard";
import CloneRepoForm from "@/components/CloneRepoForm";

export default function ProjectsPage() {
  const [instance, setInstance] = useState<Instance | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval>>();
  const [authStatus, setAuthStatus] = useState<AuthStatus>({ status: "checking", url: null });
  const authPollRef = useRef<ReturnType<typeof setInterval>>();

  const loadProjects = useCallback(async (inst: Instance) => {
    try {
      const projs = await api.getProjects(inst.id);
      setProjects(projs);
    } catch {
      // Instance may not be running
    }
  }, []);

  const loadSessions = useCallback(async (inst: Instance) => {
    try {
      const sess = await api.getSessions(inst.id);
      setSessions(sess);
    } catch {
      // Ignore polling errors
    }
  }, []);

  useEffect(() => {
    api
      .getMyInstance()
      .then((inst) => {
        setInstance(inst);
        if (inst.status === "running") {
          loadProjects(inst);
          loadSessions(inst);
          // Poll sessions every 5 seconds
          pollRef.current = setInterval(() => loadSessions(inst), 5000);
          // Poll auth status
          const pollAuth = () => api.getAuthStatus(inst.id).then(setAuthStatus).catch(() => {});
          pollAuth();
          authPollRef.current = setInterval(pollAuth, 3000);
        }
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load instance");
      })
      .finally(() => setLoading(false));

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
      if (authPollRef.current) clearInterval(authPollRef.current);
    };
  }, [loadProjects, loadSessions]);

  async function handleClone(url: string, branch?: string) {
    if (!instance) return;
    await api.cloneProject(instance.id, url, branch);
    await loadProjects(instance);
  }

  function getSessionForProject(project: Project): SessionInfo | undefined {
    return sessions.find((s) => s.project === project.name);
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

      {authStatus.status !== "authenticated" && (
        <div className="rounded-lg bg-amber-50 p-4 ring-1 ring-amber-200">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-amber-800">
                Claude Code needs authentication
              </p>
              <p className="mt-1 text-xs text-amber-600">
                Sign in to start using your projects.
              </p>
            </div>
            {authStatus.url ? (
              <a
                href={authStatus.url}
                target="_blank"
                rel="noopener noreferrer"
                className="rounded bg-amber-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-amber-700"
              >
                Sign into Claude
              </a>
            ) : (
              <span className="text-xs text-amber-500">Preparing...</span>
            )}
          </div>
        </div>
      )}

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
              <ProjectCard
                key={project.path}
                project={project}
                instance={instance}
                session={getSessionForProject(project)}
                authenticated={authStatus.status === "authenticated"}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
