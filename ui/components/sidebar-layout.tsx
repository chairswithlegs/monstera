'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

interface SidebarLayoutProps {
  navItems: Array<{ href: string; label: string }>;
  children: React.ReactNode;
}

export function SidebarLayout({ navItems, children }: SidebarLayoutProps) {
  const pathname = usePathname();

  function linkClasses(href: string) {
    return `rounded-md px-3 py-2 text-sm font-medium ${
      pathname === href
        ? 'bg-gray-100 text-gray-900'
        : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
    }`;
  }

  function mobileLinkClasses(href: string) {
    return `shrink-0 whitespace-nowrap rounded-full px-3 py-1.5 text-sm font-medium ${
      pathname === href
        ? 'bg-gray-900 text-white'
        : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
    }`;
  }

  return (
    <div className="flex md:gap-8">
      <aside className="hidden md:block w-56 shrink-0 border-r border-gray-200 bg-white py-6 px-2">
        <nav className="flex flex-col gap-1">
          {navItems.map(({ href, label }) => (
            <Link key={href} href={href} className={linkClasses(href)}>
              {label}
            </Link>
          ))}
        </nav>
      </aside>

      <div className="min-w-0 flex-1">
        <nav className="md:hidden -mx-4 md:mx-0 overflow-x-auto border-b bg-white px-4 py-2">
          <div className="flex gap-1">
            {navItems.map(({ href, label }) => (
              <Link key={href} href={href} className={mobileLinkClasses(href)}>
                {label}
              </Link>
            ))}
          </div>
        </nav>
        <main className="min-w-0 pt-4 md:pt-0">{children}</main>
      </div>
    </div>
  );
}
