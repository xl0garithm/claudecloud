"use client";

import { useRouter } from "next/navigation";
import PricingCard from "@/components/PricingCard";

export default function Home() {
  const router = useRouter();

  return (
    <div className="min-h-screen">
      {/* Hero */}
      <header className="border-b border-gray-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
          <h1 className="text-xl font-bold">Claude Cloud</h1>
          <button
            onClick={() => router.push("/auth/login")}
            className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
          >
            Sign in
          </button>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6">
        {/* Tagline */}
        <section className="py-20 text-center">
          <h2 className="text-4xl font-bold tracking-tight sm:text-5xl">
            Your personal Claude coding environment
          </h2>
          <p className="mt-4 text-lg text-gray-600">
            A persistent, cloud-hosted workspace with Claude pre-configured.
            Connect from anywhere via terminal.
          </p>
        </section>

        {/* Pricing */}
        <section className="pb-20">
          <h3 className="mb-8 text-center text-2xl font-bold">Pricing</h3>
          <div className="grid gap-8 md:grid-cols-2">
            <PricingCard
              name="Starter"
              price="$19"
              period="/month"
              features={[
                "Persistent cloud instance",
                "Claude CLI pre-installed",
                "Auto-pause on idle",
                "Connect via mosh/SSH",
                "5 GB workspace storage",
              ]}
              cta="Get Started"
              onSelect={() => router.push("/auth/login?plan=starter")}
            />
            <PricingCard
              name="Pro"
              price="$29"
              period="/month"
              highlighted
              features={[
                "Everything in Starter",
                "Priority provisioning",
                "20 GB workspace storage",
                "Usage-based add-ons",
                "Priority support",
              ]}
              cta="Go Pro"
              onSelect={() => router.push("/auth/login?plan=pro")}
            />
          </div>
        </section>
      </main>
    </div>
  );
}
