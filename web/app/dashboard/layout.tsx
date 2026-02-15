"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, User } from "@/lib/api";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    api.me().catch(() => {
      router.push("/auth/login");
    }).then((u) => {
      if (u) setUser(u);
    });
  }, [router]);

  function handleLogout() {
    document.cookie = "session=; path=/; max-age=0";
    router.push("/");
  }

  async function handleManageBilling() {
    try {
      const { url } = await api.getBillingPortal();
      window.location.href = url;
    } catch {
      // No billing account yet — ignore
    }
  }

  return (
    <div className="min-h-screen">
      <header className="border-b border-gray-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
          <h1 className="text-xl font-bold">Claude Cloud</h1>
          <div className="flex items-center gap-4">
            {user && (
              <span className="text-sm text-gray-600">{user.email}</span>
            )}
            <button
              onClick={handleManageBilling}
              className="text-sm text-gray-600 hover:text-gray-900"
            >
              Billing
            </button>
            <button
              onClick={handleLogout}
              className="text-sm text-gray-600 hover:text-gray-900"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">{children}</main>
    </div>
  );
}
