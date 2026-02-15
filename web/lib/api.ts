const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export interface User {
  id: number;
  email: string;
  name: string;
  plan: string;
  subscription_status: string;
  usage_hours: number;
}

export interface Instance {
  id: number;
  provider: string;
  provider_id: string;
  host: string;
  port: number;
  status: string;
  volume_id: string;
}

export interface UsageSummary {
  plan: string;
  subscription_status: string;
  usage_hours: number;
}

export interface FileEntry {
  name: string;
  type: "file" | "directory";
  path: string;
}

export interface Project {
  name: string;
  path: string;
  remoteUrl: string;
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
    ...options,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `API error: ${res.status}`);
  }

  return res.json();
}

export const api = {
  login(email: string) {
    return apiFetch<{ message: string }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email }),
    });
  },

  verify(token: string) {
    return apiFetch<{ token: string }>(`/auth/verify?token=${token}`, {
      headers: { Accept: "application/json" },
    });
  },

  me() {
    return apiFetch<User>("/auth/me");
  },

  getInstance(id: number) {
    return apiFetch<Instance>(`/instances/${id}`);
  },

  getMyInstance() {
    return apiFetch<Instance>("/instances/mine");
  },

  createInstance() {
    return apiFetch<Instance>("/instances", {
      method: "POST",
      body: JSON.stringify({}),
    });
  },

  pauseInstance(id: number) {
    return apiFetch<{ status: string }>(`/instances/${id}/pause`, {
      method: "POST",
    });
  },

  wakeInstance(id: number) {
    return apiFetch<{ status: string }>(`/instances/${id}/wake`, {
      method: "POST",
    });
  },

  getUsage() {
    return apiFetch<UsageSummary>("/billing/usage");
  },

  createCheckout(plan: string) {
    return apiFetch<{ url: string }>("/billing/checkout", {
      method: "POST",
      body: JSON.stringify({ plan }),
    });
  },

  getBillingPortal() {
    return apiFetch<{ url: string }>("/billing/portal");
  },

  getFiles(instanceId: number, path?: string) {
    const params = path ? `?path=${encodeURIComponent(path)}` : "";
    return apiFetch<FileEntry[]>(`/instances/${instanceId}/files${params}`);
  },

  readFile(instanceId: number, path: string) {
    return apiFetch<{ path: string; content: string; truncated: boolean }>(
      `/instances/${instanceId}/files/read?path=${encodeURIComponent(path)}`
    );
  },

  getProjects(instanceId: number) {
    return apiFetch<Project[]>(`/instances/${instanceId}/projects`);
  },

  cloneProject(instanceId: number, url: string, branch?: string) {
    return apiFetch<{ status: string; output: string }>(
      `/instances/${instanceId}/projects/clone`,
      {
        method: "POST",
        body: JSON.stringify({ url, branch: branch || undefined }),
      }
    );
  },
};
