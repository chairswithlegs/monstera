'use client';

import { getRegistrations, approveRegistration, rejectRegistration, type PendingRegistration } from '@/lib/api/admin';
import { useEffect, useState } from 'react';

export default function AdminRegistrationsPage() {
  const [pending, setPending] = useState<PendingRegistration[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);

  const load = () => {
    getRegistrations()
      .then((r) => setPending(r.pending))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  };

  useEffect(() => {
    load();
  }, []);

  const approve = async (id: string) => {
    setActing(true);
    try {
      await approveRegistration(id);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed');
    } finally {
      setActing(false);
    }
  };

  const reject = async (id: string) => {
    const reason = prompt('Rejection reason (optional):') ?? '';
    setActing(true);
    try {
      await rejectRegistration(id, reason);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed');
    } finally {
      setActing(false);
    }
  };

  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Pending registrations</h1>
      <div className="mt-6 overflow-hidden rounded-lg border border-gray-200 bg-white shadow">
        {pending.length === 0 ? (
          <p className="p-6 text-gray-500">No pending registrations.</p>
        ) : (
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Username</th>
                <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Email</th>
                <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Reason</th>
                <th className="px-4 py-2 text-right text-xs font-medium uppercase text-gray-500">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white">
              {pending.map((p) => (
                <tr key={p.user_id}>
                  <td className="px-4 py-3 text-sm font-medium text-gray-900">@{p.username}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{p.email}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{p.registration_reason ?? '—'}</td>
                  <td className="px-4 py-3 text-right text-sm">
                    <button
                      type="button"
                      disabled={acting}
                      onClick={() => approve(p.user_id)}
                      className="mr-2 text-green-600 hover:text-green-800 disabled:opacity-50"
                    >
                      Approve
                    </button>
                    <button
                      type="button"
                      disabled={acting}
                      onClick={() => reject(p.user_id)}
                      className="text-red-600 hover:text-red-800 disabled:opacity-50"
                    >
                      Reject
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
