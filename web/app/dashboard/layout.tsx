"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { api, User } from "@/lib/api";

const navTabs = [
  { label: "Overview", href: "/dashboard" },
  { label: "Terminal", href: "/dashboard/terminal" },
  { label: "Chat", href: "/dashboard/chat" },
  { label: "Projects", href: "/dashboard/projects" },
];

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
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

        {/* Navigation tabs */}
        <div className="mx-auto max-w-5xl px-6">
          <nav className="-mb-px flex gap-6">
            {navTabs.map((tab) => {
              const isActive =
                tab.href === "/dashboard"
                  ? pathname === "/dashboard"
                  : pathname?.startsWith(tab.href);
              return (
                <Link
                  key={tab.href}
                  href={tab.href}
                  className={`border-b-2 px-1 py-2 text-sm font-medium transition-colors ${
                    isActive
                      ? "border-gray-900 text-gray-900"
                      : "border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700"
                  }`}
                >
                  {tab.label}
                </Link>
              );
            })}
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">{children}</main>
    </div>
  );
}
