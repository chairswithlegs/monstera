'use client';

import { getUser, isAdminOrModerator } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import { ModeratorUserProvider } from '@/contexts/moderator-user';
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';

export default function ModeratorLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const t = useTranslations('moderator');
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getUser()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!user || !isAdminOrModerator(user)) {
      router.replace('/home');
    }
  }, [loading, user, router]);

  if (loading || !user || !isAdminOrModerator(user)) {
    return null;
  }

  const navItems = [
    { href: '/moderator/dashboard', label: t('navDashboard') },
    { href: '/moderator/users', label: t('navUsers') },
    { href: '/moderator/registrations', label: t('navRegistrations') },
    { href: '/moderator/reports', label: t('navReports') },
    { href: '/moderator/federation', label: t('navFederation') },
    { href: '/moderator/content', label: t('navContent') },
  ];

  return (
    <ModeratorUserProvider user={user}>
      <div className="flex gap-8">
        <aside className="w-56 shrink-0 border-r border-gray-200 bg-white py-6 px-2">
          <nav className="flex flex-col gap-1">
            {navItems.map(({ href, label }) => (
              <Link
                key={href}
                href={href}
                className={`rounded-md px-3 py-2 text-sm font-medium ${
                  pathname === href
                    ? 'bg-gray-100 text-gray-900'
                    : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
                }`}
              >
                {label}
              </Link>
            ))}
          </nav>
        </aside>
        <main className="min-w-0 flex-1">{children}</main>
      </div>
    </ModeratorUserProvider>
  );
}
