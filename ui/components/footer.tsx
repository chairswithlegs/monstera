'use client';

import Link from 'next/link';
import { useLocale, useTranslations } from 'next-intl';
import { locales, localeNames, type Locale } from '@/i18n/config';
import { setLocale } from '@/lib/i18n/locale';

export function Footer() {
  const t = useTranslations('footer');
  const locale = useLocale() as Locale;

  return (
    <footer className="border-t bg-white py-3">
      <div className="mx-auto flex w-full max-w-6xl items-center justify-between gap-2 px-6">
        <p className="text-sm text-muted-foreground">
          {t('poweredBy')}{' '}
          <Link
            href="https://github.com/chairswithlegs/monstera"
            target="_blank"
            rel="noopener noreferrer"
            className="underline hover:text-foreground"
          >
            Monstera
          </Link>
        </p>
        <div className="flex items-center gap-2">
          <label htmlFor="footer-locale" className="text-sm text-muted-foreground">
            {t('language')}
          </label>
          <select
            id="footer-locale"
            value={locale}
            onChange={(e) => setLocale(e.target.value as Locale)}
            className="h-8 rounded-md border border-input bg-transparent px-2 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            {locales.map((loc) => (
              <option key={loc} value={loc}>
                {localeNames[loc]}
              </option>
            ))}
          </select>
        </div>
      </div>
    </footer>
  );
}
