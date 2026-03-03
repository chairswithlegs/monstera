'use client';

import { getNodeInfo } from '@/lib/api/nodeinfo';
import { useAuth } from '@/hooks/useAuth';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import type { NodeInfoResponse } from '@/lib/api/nodeinfo';

export default function LandingPage() {
  const { token, loading: authLoading } = useAuth();
  const [nodeInfo, setNodeInfo] = useState<NodeInfoResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading) return;
    if (token) {
      window.location.replace('/home');
      return;
    }
    getNodeInfo()
      .then(setNodeInfo)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false));
  }, [token, authLoading]);

  if (authLoading || token) return null;

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50">
        <p className="text-gray-500">Loading...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-gray-50 px-4">
        <p className="text-red-600">{error}</p>
        <Link href="/login" className="text-blue-600 hover:underline">
          Sign in
        </Link>
      </div>
    );
  }

  const { software, usage, openRegistrations } = nodeInfo!;

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="mx-auto max-w-2xl px-6 py-16">
        <h1 className="text-3xl font-bold text-gray-900">{software.name}</h1>
        <p className="mt-2 text-gray-600">
          A federated server running {software.name} {software.version}.
        </p>

        <dl className="mt-8 grid gap-4 sm:grid-cols-2">
          <div className="rounded-lg border bg-white p-4">
            <dt className="text-sm font-medium text-gray-500">Users</dt>
            <dd className="mt-1 text-2xl font-semibold text-gray-900">
              {usage.users.total.toLocaleString()}
            </dd>
          </div>
          <div className="rounded-lg border bg-white p-4">
            <dt className="text-sm font-medium text-gray-500">Local posts</dt>
            <dd className="mt-1 text-2xl font-semibold text-gray-900">
              {usage.localPosts.toLocaleString()}
            </dd>
          </div>
        </dl>

        <p className="mt-4 text-sm text-gray-500">
          Registrations are {openRegistrations ? 'open' : 'closed'}.
        </p>

        <div className="mt-10 flex flex-wrap gap-4">
          <Link
            href="/login"
            className="rounded-lg bg-gray-900 px-4 py-2.5 font-medium text-white hover:bg-gray-800"
          >
            Sign in
          </Link>
          {openRegistrations && (
            <Link
              href="/register"
              className="rounded-lg border border-gray-300 bg-white px-4 py-2.5 font-medium text-gray-700 hover:bg-gray-50"
            >
              Register
            </Link>
          )}
        </div>
      </div>
    </div>
  );
}
