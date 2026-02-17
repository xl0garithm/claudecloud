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
  const [authUrl, setAuthUrl] = useState<string | null>(null);

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

  // Auth gate
  if (authStatus !== "authenticated") {
    return (
      <div className="flex h-[calc(100vh-8rem)] items-center justify-center">
        <div className="mx-auto max-w-md text-center">
          <div className="rounded-lg bg-white p-8 ring-1 ring-gray-200">
            <h2 className="text-lg font-semibold text-gray-900">
              Sign in to Claude Code
            </h2>
            <p className="mt-2 text-sm text-gray-600">
              Your instance needs to authenticate with Claude before you can use
              the terminal.
            </p>
            {authUrl ? (
              <a
                href={authUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-4 inline-block rounded bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
              >
                Sign into Claude
              </a>
            ) : (
              <div className="mt-4 flex items-center justify-center gap-2 text-sm text-gray-500">
                <svg
                  className="h-4 w-4 animate-spin"
                  viewBox="0 0 24 24"
                  fill="none"
                >
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                  />
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
                  />
                </svg>
                Preparing authentication...
              </div>
            )}
            {authStatus === "awaiting_auth" && authUrl && (
              <p className="mt-3 text-xs text-gray-400">
                Waiting for you to complete sign-in...
              </p>
            )}
          </div>
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
