'use client';

import { useAuth } from '@/hooks/useAuth';
import { getUser, isAdmin, isAdminOrModerator } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import { getNodeInfo } from '@/lib/api/nodeinfo';
import { logout } from '@/lib/auth/logout';
import { useTranslations } from 'next-intl';
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
  const t = useTranslations('nav');
  const [user, setUser] = useState<User | null>(null);
  const [serverName, setServerName] = useState<string>('');

  useEffect(() => {
    if (!loading && !token) router.replace(`/login?redirect=${encodeURIComponent(window.location.pathname + window.location.search)}`);
  }, [token, loading, router]);

  useEffect(() => {
    if (!token) return;
    getUser()
      .then(setUser)
      .catch(() => setUser(null));
  }, [token]);

  useEffect(() => {
    getNodeInfo()
      .then((info) => setServerName((info.metadata.server_name as string) || ''))
      .catch(() => {});
  }, []);

  if (loading || !token) return null;

  const showModeration = user && isAdminOrModerator(user);
  const showAdmin = user && isAdmin(user);

  return (
    <div className="flex flex-col flex-1 bg-gray-50">
      <nav className="border-b bg-white py-4">
        <div className="mx-auto flex w-full max-w-6xl items-center px-6">
          <Link href="/home" className="font-semibold text-gray-900 mr-auto">
            {serverName || 'Monstera'}
          </Link>
          <div className="flex items-center gap-6">
            <Link
              href="/account"
              className="font-medium text-gray-700 hover:text-gray-900"
            >
              {t('account')}
            </Link>
            {showModeration && (
              <Link
                href="/moderator"
                className="font-medium text-gray-700 hover:text-gray-900"
              >
                {t('moderation')}
              </Link>
            )}
            {showAdmin && (
              <Link
                href="/admin"
                className="font-medium text-gray-700 hover:text-gray-900"
              >
                {t('admin')}
              </Link>
            )}
            <button
              onClick={logout}
              className="font-medium text-gray-700 hover:text-gray-900"
            >
              {t('logout')}
            </button>
          </div>
        </div>
      </nav>
      <main className="mx-auto w-full max-w-6xl px-6 py-8">{children}</main>
    </div>
  );
}
