'use client';

import { getDashboard } from '@/lib/api/admin';
import { useEffect, useState } from 'react';

export default function AdminDashboardPage() {
  const [data, setData] = useState<Awaited<ReturnType<typeof getDashboard>> | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getDashboard()
      .then(setData)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, []);

  if (error) return <p className="text-red-600">{error}</p>;
  if (!data) return <p className="text-gray-500">Loading…</p>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Dashboard</h1>
      <div className="mt-6 grid gap-4 sm:grid-cols-3">
        <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
          <p className="text-sm font-medium text-gray-500">Local users</p>
          <p className="mt-1 text-2xl font-semibold text-gray-900">{data.local_users_count}</p>
        </div>
        <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
          <p className="text-sm font-medium text-gray-500">Local posts</p>
          <p className="mt-1 text-2xl font-semibold text-gray-900">{data.local_statuses_count}</p>
        </div>
        <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
          <p className="text-sm font-medium text-gray-500">Open reports</p>
          <p className="mt-1 text-2xl font-semibold text-gray-900">{data.open_reports_count}</p>
        </div>
      </div>
    </div>
  );
}
