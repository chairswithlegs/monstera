'use client';

import { getUsers, type AdminUser } from '@/lib/api/admin';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useState } from 'react';
import UserDetailView from './UserDetailView';

function UsersList() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getUsers({ limit: 50 })
      .then((r) => setUsers(r.users))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, []);

  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Users</h1>
      <div className="mt-6 overflow-hidden rounded-lg border border-gray-200 bg-white shadow">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Username</th>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Email</th>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Role</th>
              <th className="px-4 py-2 text-left text-xs font-medium uppercase text-gray-500">Status</th>
              <th className="px-4 py-2 text-right text-xs font-medium uppercase text-gray-500">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200 bg-white">
            {users.map((u) => (
              <tr key={u.id}>
                <td className="px-4 py-3 text-sm text-gray-900">
                  <Link href={`/admin/users?id=${encodeURIComponent(u.account_id)}`} className="font-medium text-indigo-600 hover:text-indigo-800">
                    @{u.username}
                  </Link>
                </td>
                <td className="px-4 py-3 text-sm text-gray-600">{u.email}</td>
                <td className="px-4 py-3 text-sm text-gray-600">{u.role}</td>
                <td className="px-4 py-3 text-sm">
                  {u.suspended && <span className="rounded bg-red-100 px-2 py-0.5 text-red-800">Suspended</span>}
                  {u.silenced && !u.suspended && <span className="rounded bg-amber-100 px-2 py-0.5 text-amber-800">Silenced</span>}
                  {!u.suspended && !u.silenced && <span className="text-gray-500">—</span>}
                </td>
                <td className="px-4 py-3 text-right text-sm">
                  <Link href={`/admin/users?id=${encodeURIComponent(u.account_id)}`} className="text-indigo-600 hover:text-indigo-800">
                    View
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default function AdminUsersPage() {
  const searchParams = useSearchParams();
  const id = searchParams.get('id');

  if (id) {
    return <UserDetailView id={id} />;
  }

  return (
    <Suspense fallback={<p className="text-gray-500">Loading…</p>}>
      <UsersList />
    </Suspense>
  );
}
