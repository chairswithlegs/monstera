'use client';

import { useTranslations } from 'next-intl';
import { SidebarLayout } from '@/components/sidebar-layout';

export default function AccountLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const t = useTranslations('account');

  const navItems = [
    { href: '/account/profile', label: t('navProfile') },
    { href: '/account/preferences', label: t('navPreferences') },
    { href: '/account/security', label: t('navSecurity') },
  ];

  return <SidebarLayout navItems={navItems}>{children}</SidebarLayout>;
}
