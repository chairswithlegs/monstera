'use client';

import { getUser, isAdmin } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import { useTranslations } from 'next-intl';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { SidebarLayout } from '@/components/sidebar-layout';

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const t = useTranslations('admin');
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
    if (!user || !isAdmin(user)) {
      router.replace('/home');
    }
  }, [loading, user, router]);

  if (loading || !user || !isAdmin(user)) {
    return null;
  }

  const navItems = [
    { href: '/admin/dashboard', label: t('navAdminDashboard') },
    { href: '/admin/settings', label: t('navSettings') },
    { href: '/admin/users', label: t('navUsers') },
  ];

  return <SidebarLayout navItems={navItems}>{children}</SidebarLayout>;
}
