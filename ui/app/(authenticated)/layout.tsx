'use client';

import { useAuth } from '@/hooks/useAuth';
import { getUser, isAdmin, isAdminOrModerator } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';

export default function AuthenticatedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { token, loading } = useAuth();
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    if (!loading && !token) router.replace('/login');
  }, [token, loading, router]);

  useEffect(() => {
    if (!token) return;
    getUser()
      .then(setUser)
      .catch(() => setUser(null));
  }, [token]);

  if (loading || !token) return null;

  const showModeration = user && isAdminOrModerator(user);
  const showAdmin = user && isAdmin(user);

  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="border-b bg-white px-6 py-4">
        <div className="mx-auto flex max-w-6xl items-center gap-6">
          <Link
            href="/home"
            className="font-medium text-gray-900 hover:text-gray-700"
          >
            Home
          </Link>
          <Link
            href="/settings"
            className="font-medium text-gray-700 hover:text-gray-900"
          >
            Account settings
          </Link>
          {showModeration && (
            <Link
              href="/moderator"
              className="font-medium text-gray-700 hover:text-gray-900"
            >
              Moderation
            </Link>
          )}
          {showAdmin && (
            <Link
              href="/admin"
              className="font-medium text-gray-700 hover:text-gray-900"
            >
              Admin
            </Link>
          )}
        </div>
      </nav>
      <main className="mx-auto max-w-6xl px-6 py-8">{children}</main>
    </div>
  );
}
