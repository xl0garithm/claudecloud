"use client";

import { useEffect, useState } from "react";
import { api, Settings } from "@/lib/api";

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [oauthToken, setOauthToken] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    api.getSettings().then(setSettings).catch(() => {});
  }, []);

  async function saveAndRefresh(fields: { anthropic_api_key?: string; claude_oauth_token?: string }) {
    setSaving(true);
    setMessage("");
    setError("");
    try {
      await api.updateSettings(fields);
      setMessage("Saved. Re-provision your instance for changes to take effect.");
      setOauthToken("");
      setApiKey("");
      setSettings(await api.getSettings());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  }

  const authBadge = settings?.auth_method === "oauth"
    ? { label: "Claude Max/Pro", color: "bg-purple-100 text-purple-800" }
    : settings?.auth_method === "api_key"
    ? { label: "API Key", color: "bg-blue-100 text-blue-800" }
    : { label: "Not configured", color: "bg-yellow-100 text-yellow-800" };

  return (
    <div className="space-y-8">
      <h2 className="text-2xl font-bold">Settings</h2>

      {/* Auth status */}
      <div className="rounded-xl bg-white p-6 ring-1 ring-gray-200">
        <div className="flex items-center gap-3">
          <h3 className="text-lg font-semibold">Claude Code Authentication</h3>
          <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${authBadge.color}`}>
            {authBadge.label}
          </span>
        </div>
        <p className="mt-1 text-sm text-gray-600">
          Configure how Claude Code authenticates in your instance. Choose one method below.
        </p>
        {message && <p className="mt-3 text-sm text-green-700">{message}</p>}
        {error && <p className="mt-3 text-sm text-red-700">{error}</p>}
      </div>

      {/* Option 1: OAuth Token (Max/Pro plan) */}
      <div className="rounded-xl bg-white p-6 ring-1 ring-gray-200">
        <h3 className="font-semibold">Option 1: Claude Max/Pro Login Token</h3>
        <p className="mt-1 text-sm text-gray-600">
          Uses your Claude subscription (no separate API charges). Generate a
          token by running <code className="rounded bg-gray-100 px-1.5 py-0.5 text-xs">claude setup-token</code> on
          any machine with Claude Code installed, then paste it here.
        </p>

        {settings?.claude_oauth_token && (
          <div className="mt-4 flex items-center gap-3">
            <span className="text-sm text-gray-500">Current token:</span>
            <code className="rounded bg-gray-100 px-2 py-1 text-sm font-mono">
              {settings.claude_oauth_token}
            </code>
            <button
              onClick={() => saveAndRefresh({ claude_oauth_token: "" })}
              disabled={saving}
              className="text-sm text-red-600 hover:text-red-800 disabled:opacity-50"
            >
              Remove
            </button>
          </div>
        )}

        <div className="mt-4 flex gap-3">
          <input
            type="password"
            value={oauthToken}
            onChange={(e) => setOauthToken(e.target.value)}
            placeholder="sk-ant-oat01-..."
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm focus:border-gray-500 focus:outline-none focus:ring-1 focus:ring-gray-500"
          />
          <button
            onClick={() => saveAndRefresh({ claude_oauth_token: oauthToken })}
            disabled={saving || !oauthToken}
            className="rounded-lg bg-purple-700 px-6 py-2 text-sm font-medium text-white hover:bg-purple-600 disabled:opacity-50"
          >
            {saving ? "Saving..." : "Save Token"}
          </button>
        </div>
      </div>

      {/* Option 2: API Key */}
      <div className="rounded-xl bg-white p-6 ring-1 ring-gray-200">
        <h3 className="font-semibold">Option 2: Anthropic API Key</h3>
        <p className="mt-1 text-sm text-gray-600">
          Pay-as-you-go API billing. Get a key from{" "}
          <a
            href="https://console.anthropic.com/settings/keys"
            target="_blank"
            rel="noopener noreferrer"
            className="text-blue-600 hover:underline"
          >
            console.anthropic.com
          </a>
          .
        </p>

        {settings?.anthropic_api_key && (
          <div className="mt-4 flex items-center gap-3">
            <span className="text-sm text-gray-500">Current key:</span>
            <code className="rounded bg-gray-100 px-2 py-1 text-sm font-mono">
              {settings.anthropic_api_key}
            </code>
            <button
              onClick={() => saveAndRefresh({ anthropic_api_key: "" })}
              disabled={saving}
              className="text-sm text-red-600 hover:text-red-800 disabled:opacity-50"
            >
              Remove
            </button>
          </div>
        )}

        <div className="mt-4 flex gap-3">
          <input
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder="sk-ant-api03-..."
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm focus:border-gray-500 focus:outline-none focus:ring-1 focus:ring-gray-500"
          />
          <button
            onClick={() => saveAndRefresh({ anthropic_api_key: apiKey })}
            disabled={saving || !apiKey}
            className="rounded-lg bg-gray-900 px-6 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50"
          >
            {saving ? "Saving..." : "Save Key"}
          </button>
        </div>
      </div>

      <p className="text-xs text-gray-400">
        If both are set, the OAuth token takes priority (uses your Max/Pro
        subscription). Credentials are injected as environment variables when
        your instance is created. Re-provision your instance after changing
        credentials.
      </p>
    </div>
  );
}
