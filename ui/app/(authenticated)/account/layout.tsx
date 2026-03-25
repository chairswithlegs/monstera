'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useTranslations } from 'next-intl';

export default function AccountLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const t = useTranslations('account');

  const navItems = [
    { href: '/account/profile', label: t('navProfile') },
    { href: '/account/preferences', label: t('navPreferences') },
    { href: '/account/security', label: t('navSecurity') },
  ];

  return (
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
  );
}
