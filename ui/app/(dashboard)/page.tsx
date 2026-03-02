'use client';

import { getUser } from "@/lib/api/user";
import { useEffect, useState } from "react";
import type { User } from "@/lib/api/user";

export default function DashboardPage() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getUser()
      .then(setUser)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div>Loading...</div>;
  if (error) return <div className="text-red-600">{error}</div>;
  if (!user) return <div>Loading...</div>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Dashboard</h1>
      <p className="mt-2 text-gray-500">Welcome to your Mastodon admin panel.</p>
      <pre className="mt-4 p-4 bg-gray-100 rounded-lg">
        <code className="text-sm">{JSON.stringify(user, null, 2)}</code>
      </pre>
    </div>
  );
}