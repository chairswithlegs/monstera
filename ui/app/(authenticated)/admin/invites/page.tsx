'use client';

import { getInvites, createInvite, deleteInvite, type Invite } from '@/lib/api/admin';
import { useEffect, useState } from 'react';

export default function AdminInvitesPage() {
  const [invites, setInvites] = useState<Invite[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const load = () => {
    getInvites()
      .then((r) => setInvites(r.invites))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  };

  useEffect(() => {
    load();
  }, []);

  const create = async () => {
    setCreating(true);
    try {
      await createInvite({});
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create');
    } finally {
      setCreating(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await deleteInvite(id);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete');
    }
  };

  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Invites</h1>
      <div className="mt-4">
        <button
          type="button"
          disabled={creating}
          onClick={create}
          className="rounded bg-indigo-600 px-3 py-1.5 text-sm text-white hover:bg-indigo-500 disabled:opacity-50"
        >
          Create invite
        </button>
      </div>
      <div className="mt-6 overflow-hidden rounded-lg border border-gray-200 bg-white shadow">
        {invites.length === 0 ? (
          <p className="p-6 text-gray-500">No invites yet.</p>
        ) : (
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Code</th>
                <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Uses</th>
                <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Created</th>
                <th className="px-4 py-2 text-right text-xs font-medium uppercase text-gray-500">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white">
              {invites.map((inv) => (
                <tr key={inv.id}>
                  <td className="px-4 py-3 font-mono text-sm text-gray-900">{inv.code}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">
                    {inv.uses}
                    {inv.max_uses != null ? ` / ${inv.max_uses}` : ''}
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-600">{new Date(inv.created_at).toLocaleString()}</td>
                  <td className="px-4 py-3 text-right">
                    <button
                      type="button"
                      onClick={() => remove(inv.id)}
                      className="text-red-600 hover:text-red-800"
                    >
                      Revoke
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
