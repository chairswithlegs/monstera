'use client';

import { getUser, isAdminOrModerator } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import { ModeratorUserProvider } from '@/contexts/moderator-user';
import { useTranslations } from 'next-intl';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { SidebarLayout } from '@/components/sidebar-layout';

export default function ModeratorLayout({
  children,
}: {
  children: React.ReactNode;
}) {
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
    { href: '/moderator/dashboard', label: t('navModeratorDashboard') },
    { href: '/moderator/users', label: t('navUsers') },
    { href: '/moderator/registrations', label: t('navRegistrations') },
    { href: '/moderator/reports', label: t('navReports') },
    { href: '/moderator/federation', label: t('navFederation') },
    { href: '/moderator/content', label: t('navContent') },
  ];

  return (
    <ModeratorUserProvider user={user}>
      <SidebarLayout navItems={navItems}>{children}</SidebarLayout>
    </ModeratorUserProvider>
  );
}
