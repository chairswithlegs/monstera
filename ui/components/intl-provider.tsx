'use client';

import { useState, useEffect } from 'react';
import { NextIntlClientProvider } from 'next-intl';
import { getLocaleFromCookie } from '@/lib/i18n/locale';
import { defaultLocale, type Locale } from '@/i18n/config';

interface Props {
  children: React.ReactNode;
  messages: Record<string, Record<string, unknown>>;
}

export function IntlProvider({ children, messages }: Props) {
  const [locale, setLocale] = useState<Locale>(defaultLocale);

  useEffect(() => {
    const detected = getLocaleFromCookie();
    setLocale(detected);
    document.documentElement.lang = detected;
  }, []);

  const localeMessages =
    messages[locale] ?? messages[defaultLocale] ?? {};

  return (
    <NextIntlClientProvider locale={locale} messages={localeMessages}>
      {children}
    </NextIntlClientProvider>
  );
}
