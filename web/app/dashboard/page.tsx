"use client";

import { useEffect, useState, useCallback } from "react";
import { api, User, Instance, UsageSummary } from "@/lib/api";
import InstanceCard from "@/components/InstanceCard";

export default function DashboardPage() {
  const [user, setUser] = useState<User | null>(null);
  const [instance, setInstance] = useState<Instance | null>(null);
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const loadData = useCallback(async () => {
    try {
      const u = await api.me();
      setUser(u);

      // Try to load usage
      api.getUsage().then(setUsage).catch(() => {});

      // Load user's instance (if any)
      api.getMyInstance().then(setInstance).catch(() => {});
    } catch {
      setError("Failed to load data");
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  async function handleProvision() {
    setLoading(true);
    setError("");
    try {
      const inst = await api.createInstance();
      setInstance(inst);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to provision");
    } finally {
      setLoading(false);
    }
  }

  async function handlePause() {
    if (!instance) return;
    setLoading(true);
    try {
      await api.pauseInstance(instance.id);
      setInstance({ ...instance, status: "stopped" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to pause");
    } finally {
      setLoading(false);
    }
  }

  async function handleWake() {
    if (!instance) return;
    setLoading(true);
    try {
      await api.wakeInstance(instance.id);
      setInstance({ ...instance, status: "running" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to wake");
    } finally {
      setLoading(false);
    }
  }

  async function handleCheckout(plan: string) {
    try {
      const { url } = await api.createCheckout(plan);
      window.location.href = url;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start checkout");
    }
  }

  return (
    <div className="space-y-8">
      <h2 className="text-2xl font-bold">Dashboard</h2>

      {error && (
        <div className="rounded-lg bg-red-50 p-4 text-sm text-red-700">
          {error}
        </div>
      )}

      {/* Subscription status */}
      {user && (
        <div className="rounded-xl bg-white p-6 ring-1 ring-gray-200">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="font-semibold">Plan: {user.plan}</h3>
              <p className="text-sm text-gray-600">
                Status: {user.subscription_status}
              </p>
            </div>
            {usage && (
              <div className="text-right">
                <p className="text-2xl font-bold">
                  {usage.usage_hours.toFixed(1)}h
                </p>
                <p className="text-sm text-gray-600">usage this period</p>
              </div>
            )}
          </div>

          {user.subscription_status !== "active" && (
            <div className="mt-4 flex gap-3">
              <button
                onClick={() => handleCheckout("starter")}
                className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
              >
                Subscribe - Starter $19/mo
              </button>
              <button
                onClick={() => handleCheckout("pro")}
                className="rounded-lg bg-gray-700 px-4 py-2 text-sm font-medium text-white hover:bg-gray-600"
              >
                Subscribe - Pro $29/mo
              </button>
            </div>
          )}
        </div>
      )}

      {/* Instance */}
      {instance ? (
        <InstanceCard
          instance={instance}
          onPause={handlePause}
          onWake={handleWake}
          loading={loading}
        />
      ) : (
        <div className="rounded-xl bg-white p-6 text-center ring-1 ring-gray-200">
          <p className="text-gray-600">No instance provisioned yet.</p>
          <button
            onClick={handleProvision}
            disabled={loading}
            className="mt-4 rounded-lg bg-green-600 px-6 py-2 font-medium text-white hover:bg-green-700 disabled:opacity-50"
          >
            {loading ? "Provisioning..." : "Provision Instance"}
          </button>
        </div>
      )}
    </div>
  );
}
