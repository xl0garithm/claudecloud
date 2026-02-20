"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { api, Instance } from "@/lib/api";
import WebTerminal from "@/components/WebTerminal";

function TerminalContent() {
  const searchParams = useSearchParams();
  const tab = searchParams.get("tab");
  const [instance, setInstance] = useState<Instance | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [authStatus, setAuthStatus] = useState<string>("checking");
  const [, setAuthUrl] = useState<string | null>(null);

  useEffect(() => {
    api
      .getMyInstance()
      .then((inst) => {
        setInstance(inst);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load instance");
      })
      .finally(() => setLoading(false));
  }, []);

  // Poll auth status
  useEffect(() => {
    if (!instance || instance.status !== "running") return;

    let cancelled = false;
    function poll() {
      if (cancelled) return;
      api.getAuthStatus(instance!.id).then((auth) => {
        if (cancelled) return;
        setAuthStatus(auth.status);
        setAuthUrl(auth.url);
      }).catch(() => {});
    }

    poll();
    const interval = setInterval(poll, 3000);
    return () => { cancelled = true; clearInterval(interval); };
  }, [instance]);

  // Switch tab after auth is confirmed
  useEffect(() => {
    if (authStatus === "authenticated" && instance && tab) {
      api.createTab(instance.id, tab, tab).catch(() => {});
    }
  }, [authStatus, instance, tab]);

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
    <div className="flex h-[calc(100vh-8rem)] flex-col">
      {authStatus !== "authenticated" && (
        <div className="rounded-lg bg-amber-50 px-4 py-3 ring-1 ring-amber-200">
          <p className="text-sm font-medium text-amber-800">
            Authentication required.{" "}
            <span className="font-normal text-amber-700">
              Accept the trust prompt in the terminal below, then type{" "}
              <code className="rounded bg-amber-100 px-1 py-0.5 text-xs font-mono">/login</code>{" "}
              to sign in.
            </span>
          </p>
        </div>
      )}
      <div className="flex-1 overflow-hidden rounded-lg ring-1 ring-gray-200">
        <WebTerminal instanceId={instance.id} />
      </div>
    </div>
  );
}

export default function TerminalPage() {
  return (
    <Suspense
      fallback={
        <div className="flex h-[calc(100vh-8rem)] items-center justify-center">
          <div className="text-gray-500">Loading instance...</div>
        </div>
      }
    >
      <TerminalContent />
    </Suspense>
  );
}
