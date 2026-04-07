'use client';

import { useAuth } from '@/hooks/useAuth';
import { getUser, isAdmin, isAdminOrModerator } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import { getNodeInfo } from '@/lib/api/nodeinfo';
import { logout } from '@/lib/auth/logout';
import { useTranslations } from 'next-intl';
import { Menu } from 'lucide-react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import {
  Sheet,
  SheetContent,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet';

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
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

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
        <div className="mx-auto flex w-full max-w-6xl items-center px-4 md:px-6">
          <Link href="/home" className="font-semibold text-gray-900 mr-auto">
            {serverName || 'Monstera'}
          </Link>
          <div className="hidden md:flex items-center gap-6">
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
          <Sheet open={mobileNavOpen} onOpenChange={setMobileNavOpen}>
            <SheetTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="md:hidden"
                aria-label={t('openMenu')}
              >
                <Menu className="h-5 w-5" />
              </Button>
            </SheetTrigger>
            <SheetContent side="right">
              <SheetTitle className="sr-only">{t('navigation')}</SheetTitle>
              <nav className="flex flex-col gap-1 px-2 pt-8">
                <Link
                  href="/account"
                  onClick={() => setMobileNavOpen(false)}
                  className="rounded-md px-3 py-2 text-sm font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900"
                >
                  {t('account')}
                </Link>
                {showModeration && (
                  <Link
                    href="/moderator"
                    onClick={() => setMobileNavOpen(false)}
                    className="rounded-md px-3 py-2 text-sm font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900"
                  >
                    {t('moderation')}
                  </Link>
                )}
                {showAdmin && (
                  <Link
                    href="/admin"
                    onClick={() => setMobileNavOpen(false)}
                    className="rounded-md px-3 py-2 text-sm font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900"
                  >
                    {t('admin')}
                  </Link>
                )}
                <button
                  onClick={() => {
                    setMobileNavOpen(false);
                    logout();
                  }}
                  className="rounded-md px-3 py-2 text-left text-sm font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900"
                >
                  {t('logout')}
                </button>
              </nav>
            </SheetContent>
          </Sheet>
        </div>
      </nav>
      <main className="mx-auto w-full max-w-6xl px-4 py-4 md:px-6 md:py-8">{children}</main>
    </div>
  );
}
