"use client";

import { Instance } from "@/lib/api";

interface InstanceCardProps {
  instance: Instance;
  onPause: () => void;
  onWake: () => void;
  loading?: boolean;
}

const statusColors: Record<string, string> = {
  running: "bg-green-100 text-green-800",
  stopped: "bg-yellow-100 text-yellow-800",
  provisioning: "bg-blue-100 text-blue-800",
  destroyed: "bg-red-100 text-red-800",
};

export default function InstanceCard({
  instance,
  onPause,
  onWake,
  loading,
}: InstanceCardProps) {
  const colorClass = statusColors[instance.status] || "bg-gray-100 text-gray-800";

  return (
    <div className="rounded-xl bg-white p-6 ring-1 ring-gray-200">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold">Your Instance</h3>
        <span className={`rounded-full px-3 py-1 text-sm font-medium ${colorClass}`}>
          {instance.status}
        </span>
      </div>

      <div className="mt-4 space-y-2 text-sm text-gray-600">
        <div className="flex justify-between">
          <span>Provider</span>
          <span className="font-mono">{instance.provider}</span>
        </div>
        <div className="flex justify-between">
          <span>ID</span>
          <span className="font-mono">{instance.provider_id}</span>
        </div>
        {instance.host && (
          <div className="flex justify-between">
            <span>Host</span>
            <span className="font-mono">{instance.host}</span>
          </div>
        )}
      </div>

      {instance.status === "running" && (
        <div className="mt-4">
          <p className="mb-2 text-sm font-medium text-gray-700">Connect:</p>
          <code className="block rounded bg-gray-900 p-3 text-sm text-green-400">
            curl -fsSL {process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}
            /connect.sh?user_id={instance.id} | bash
          </code>
        </div>
      )}

      <div className="mt-4 flex gap-3">
        {instance.status === "running" && (
          <button
            onClick={onPause}
            disabled={loading}
            className="rounded-lg bg-yellow-500 px-4 py-2 text-sm font-medium text-white transition hover:bg-yellow-600 disabled:opacity-50"
          >
            {loading ? "..." : "Pause"}
          </button>
        )}
        {instance.status === "stopped" && (
          <button
            onClick={onWake}
            disabled={loading}
            className="rounded-lg bg-green-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-green-700 disabled:opacity-50"
          >
            {loading ? "..." : "Wake"}
          </button>
        )}
      </div>
    </div>
  );
}
