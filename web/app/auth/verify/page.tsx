"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { Suspense } from "react";

function VerifyContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [error, setError] = useState("");

  useEffect(() => {
    const token = searchParams.get("token");
    if (!token) {
      setError("Missing verification token");
      return;
    }

    api
      .verify(token)
      .then(() => {
        router.push("/dashboard");
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Verification failed");
      });
  }, [searchParams, router]);

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="w-full max-w-sm rounded-xl bg-white p-8 text-center ring-1 ring-gray-200">
          <h2 className="text-xl font-bold text-red-600">Verification Failed</h2>
          <p className="mt-2 text-gray-600">{error}</p>
          <button
            onClick={() => router.push("/auth/login")}
            className="mt-4 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
          >
            Try again
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="w-full max-w-sm rounded-xl bg-white p-8 text-center ring-1 ring-gray-200">
        <h2 className="text-xl font-bold">Verifying...</h2>
        <p className="mt-2 text-gray-600">Please wait while we sign you in.</p>
      </div>
    </div>
  );
}

export default function VerifyPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center">
          <p>Loading...</p>
        </div>
      }
    >
      <VerifyContent />
    </Suspense>
  );
}
