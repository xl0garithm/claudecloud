"use client";

import { useEffect, useState } from "react";
import { api, Instance } from "@/lib/api";
import WebTerminal from "@/components/WebTerminal";

export default function TerminalPage() {
  const [instance, setInstance] = useState<Instance | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .getMyInstance()
      .then(setInstance)
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load instance");
      })
      .finally(() => setLoading(false));
  }, []);

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
          <p className="text-gray-600">
            {error || "No active instance found."}
          </p>
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
            connect.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-[calc(100vh-8rem)] overflow-hidden rounded-lg ring-1 ring-gray-200">
      <WebTerminal instanceId={instance.id} />
    </div>
  );
}
