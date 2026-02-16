"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import PricingCard from "@/components/PricingCard";

function FAQItem({ question, answer }: { question: string; answer: string }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="border-b border-gray-200">
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center justify-between py-5 text-left"
      >
        <span className="text-base font-medium text-gray-900">{question}</span>
        <span className="ml-4 text-gray-400">{open ? "\u2212" : "+"}</span>
      </button>
      {open && (
        <p className="pb-5 text-sm leading-relaxed text-gray-600">{answer}</p>
      )}
    </div>
  );
}

const FEATURES = [
  {
    title: "Web Terminal",
    description:
      "Full terminal access from your browser. Powered by xterm.js and ttyd with persistent Zellij sessions.",
    icon: "\u2588\u2591",
  },
  {
    title: "Chat UI",
    description:
      "Chat with Claude directly in the browser. Streaming responses, tool use visibility, and markdown rendering.",
    icon: "\u2728",
  },
  {
    title: "Project Management",
    description:
      "Clone repos, browse files, and jump between projects. Your workspace persists across sessions.",
    icon: "\uD83D\uDCC2",
  },
  {
    title: "Auto-Pause",
    description:
      "Instances pause automatically when idle and wake on demand. You only pay for what you use.",
    icon: "\u23F8",
  },
];

const STEPS = [
  {
    step: "1",
    title: "Sign up & pick a plan",
    description: "Create an account with your email. Choose Starter or Pro.",
  },
  {
    step: "2",
    title: "Your instance provisions",
    description:
      "A persistent cloud workspace spins up with Claude Code pre-installed and ready to go.",
  },
  {
    step: "3",
    title: "Start building",
    description:
      "Open the web terminal, chat with Claude, or connect via your local terminal. Your workspace persists.",
  },
];

const FAQS = [
  {
    question: "What is Claude Cloud?",
    answer:
      "Claude Cloud gives you a persistent, cloud-hosted development environment with Claude Code pre-installed. Think of it as your always-on AI pair programmer running in the cloud, accessible from any browser or terminal.",
  },
  {
    question: "How do I connect to my instance?",
    answer:
      "You can use the built-in web terminal, chat UI, or connect from your local terminal via mosh/SSH. Your Zellij session persists across connections, so you never lose context.",
  },
  {
    question: "What happens when my instance is idle?",
    answer:
      "Instances auto-pause after 2 hours of inactivity to save resources. When you come back, just hit 'Wake' and your workspace resumes exactly where you left off — all files and projects are preserved on persistent storage.",
  },
  {
    question: "Can I use my own Anthropic API key?",
    answer:
      "Yes. Your API key is securely injected into your instance environment. Claude Code uses it directly for all interactions.",
  },
  {
    question: "Is my data secure?",
    answer:
      "Each user gets an isolated environment with their own persistent volume. Network access is controlled via zero-trust networking, and all API connections are authenticated with JWT tokens.",
  },
];

export default function Home() {
  const router = useRouter();

  return (
    <div className="min-h-screen bg-white">
      {/* Nav */}
      <header className="border-b border-gray-200">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
          <h1 className="text-xl font-bold">Claude Cloud</h1>
          <nav className="flex items-center gap-6">
            <a href="#features" className="text-sm text-gray-600 hover:text-gray-900">
              Features
            </a>
            <a href="#pricing" className="text-sm text-gray-600 hover:text-gray-900">
              Pricing
            </a>
            <a href="#faq" className="text-sm text-gray-600 hover:text-gray-900">
              FAQ
            </a>
            <button
              onClick={() => router.push("/auth/login")}
              className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
            >
              Sign in
            </button>
          </nav>
        </div>
      </header>

      <main>
        {/* Hero */}
        <section className="mx-auto max-w-5xl px-6 py-24 text-center">
          <h2 className="text-4xl font-bold tracking-tight sm:text-5xl lg:text-6xl">
            Your personal Claude coding
            <br />
            environment in the cloud
          </h2>
          <p className="mx-auto mt-6 max-w-2xl text-lg text-gray-600">
            A persistent, always-ready workspace with Claude Code pre-configured.
            Open your browser, start building. Terminal, chat, and project management
            built in.
          </p>
          <div className="mt-10 flex items-center justify-center gap-4">
            <button
              onClick={() => router.push("/auth/login?plan=starter")}
              className="rounded-lg bg-gray-900 px-6 py-3 text-sm font-medium text-white hover:bg-gray-800"
            >
              Get started
            </button>
            <a
              href="#how-it-works"
              className="rounded-lg border border-gray-300 px-6 py-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              How it works
            </a>
          </div>
        </section>

        {/* How it works */}
        <section id="how-it-works" className="border-t border-gray-100 bg-gray-50 py-20">
          <div className="mx-auto max-w-5xl px-6">
            <h3 className="text-center text-2xl font-bold">How it works</h3>
            <div className="mt-12 grid gap-8 md:grid-cols-3">
              {STEPS.map((s) => (
                <div key={s.step} className="text-center">
                  <div className="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-gray-900 text-sm font-bold text-white">
                    {s.step}
                  </div>
                  <h4 className="mt-4 text-lg font-semibold">{s.title}</h4>
                  <p className="mt-2 text-sm text-gray-600">{s.description}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* Features */}
        <section id="features" className="py-20">
          <div className="mx-auto max-w-5xl px-6">
            <h3 className="text-center text-2xl font-bold">Everything you need</h3>
            <p className="mx-auto mt-3 max-w-xl text-center text-gray-600">
              A complete cloud development environment powered by Claude.
            </p>
            <div className="mt-12 grid gap-8 sm:grid-cols-2">
              {FEATURES.map((f) => (
                <div
                  key={f.title}
                  className="rounded-xl border border-gray-200 p-6"
                >
                  <span className="text-2xl">{f.icon}</span>
                  <h4 className="mt-3 text-lg font-semibold">{f.title}</h4>
                  <p className="mt-2 text-sm leading-relaxed text-gray-600">
                    {f.description}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* Pricing */}
        <section id="pricing" className="border-t border-gray-100 bg-gray-50 py-20">
          <div className="mx-auto max-w-5xl px-6">
            <h3 className="text-center text-2xl font-bold">Simple pricing</h3>
            <p className="mx-auto mt-3 max-w-xl text-center text-gray-600">
              Start building today. Upgrade when you need more.
            </p>
            <div className="mt-12 grid gap-8 md:grid-cols-2">
              <PricingCard
                name="Starter"
                price="$19"
                period="/month"
                features={[
                  "Persistent cloud instance",
                  "Claude CLI pre-installed",
                  "Auto-pause on idle",
                  "Connect via browser or terminal",
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
                badge="Most Popular"
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
          </div>
        </section>

        {/* FAQ */}
        <section id="faq" className="py-20">
          <div className="mx-auto max-w-3xl px-6">
            <h3 className="text-center text-2xl font-bold">
              Frequently asked questions
            </h3>
            <div className="mt-12">
              {FAQS.map((faq) => (
                <FAQItem
                  key={faq.question}
                  question={faq.question}
                  answer={faq.answer}
                />
              ))}
            </div>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="border-t border-gray-200 bg-white py-10">
        <div className="mx-auto flex max-w-5xl flex-col items-center justify-between gap-4 px-6 sm:flex-row">
          <p className="text-sm text-gray-500">Claude Cloud</p>
          <div className="flex gap-6">
            <a href="#features" className="text-sm text-gray-500 hover:text-gray-700">
              Features
            </a>
            <a href="#pricing" className="text-sm text-gray-500 hover:text-gray-700">
              Pricing
            </a>
            <a href="#faq" className="text-sm text-gray-500 hover:text-gray-700">
              FAQ
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
