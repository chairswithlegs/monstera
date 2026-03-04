'use client';

import {
  getUser,
  suspendUser,
  unsuspendUser,
  silenceUser,
  unsilenceUser,
  setUserRole,
  deleteUser,
  type AdminUser,
} from '@/lib/api/admin';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useState } from 'react';

type Props = { id: string };

export default function UserDetailView({ id }: Props) {
  const router = useRouter();
  const [user, setUser] = useState<AdminUser | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);

  const load = useCallback(() => {
    if (!id) return;
    getUser(id)
      .then(setUser)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const act = async (fn: () => Promise<void>) => {
    setActing(true);
    try {
      await fn();
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Action failed');
    } finally {
      setActing(false);
    }
  };

  if (error && !user) return <p className="text-red-600">{error}</p>;
  if (!user) return <p className="text-gray-500">Loading…</p>;

  return (
    <div>
      <p className="mb-4">
        <Link href="/admin/users" className="text-indigo-600 hover:text-indigo-800">
          ← Back to users
        </Link>
      </p>
      <h1 className="text-2xl font-semibold text-gray-900">@{user.username}</h1>
      <p className="mt-1 text-gray-600">{user.email}</p>
      <p className="mt-1 text-sm text-gray-500">Account ID: {user.account_id}</p>
      <p className="mt-1 text-sm text-gray-500">Role: {user.role}</p>
      {user.suspended && <p className="mt-2 text-red-600">Suspended</p>}
      {user.silenced && !user.suspended && <p className="mt-2 text-amber-600">Silenced</p>}
      {error && <p className="mt-2 text-red-600">{error}</p>}
      <div className="mt-6 flex flex-wrap gap-2">
        {user.suspended ? (
          <button
            type="button"
            disabled={acting}
            onClick={() => act(() => unsuspendUser(user.account_id))}
            className="rounded bg-gray-800 px-3 py-1.5 text-sm text-white hover:bg-gray-700 disabled:opacity-50"
          >
            Unsuspend
          </button>
        ) : (
          <button
            type="button"
            disabled={acting}
            onClick={() => act(() => suspendUser(user.account_id))}
            className="rounded bg-red-600 px-3 py-1.5 text-sm text-white hover:bg-red-500 disabled:opacity-50"
          >
            Suspend
          </button>
        )}
        {user.silenced ? (
          <button
            type="button"
            disabled={acting}
            onClick={() => act(() => unsilenceUser(user.account_id))}
            className="rounded bg-gray-800 px-3 py-1.5 text-sm text-white hover:bg-gray-700 disabled:opacity-50"
          >
            Unsilence
          </button>
        ) : (
          <button
            type="button"
            disabled={acting}
            onClick={() => act(() => silenceUser(user.account_id))}
            className="rounded bg-amber-600 px-3 py-1.5 text-sm text-white hover:bg-amber-500 disabled:opacity-50"
          >
            Silence
          </button>
        )}
        <button
          type="button"
          disabled={acting}
          onClick={() => {
            const role = prompt('New role (user, moderator, admin):', user.role);
            if (role) act(() => setUserRole(user.account_id, role));
          }}
          className="rounded bg-gray-600 px-3 py-1.5 text-sm text-white hover:bg-gray-500 disabled:opacity-50"
        >
          Set role
        </button>
        <button
          type="button"
          disabled={acting}
          onClick={() => {
            if (confirm('Permanently delete this account?'))
              act(() => deleteUser(user.account_id)).then(() => router.push('/admin/users'));
          }}
          className="rounded bg-red-800 px-3 py-1.5 text-sm text-white hover:bg-red-700 disabled:opacity-50"
        >
          Delete account
        </button>
      </div>
    </div>
  );
}
