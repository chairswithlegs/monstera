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
  // Lazy initializer reads the cookie synchronously on the client so the correct
  // locale is applied on the very first render, avoiding a flash of the default
  // locale. On the server (static build) getLocaleFromCookie() returns the
  // default; suppressHydrationWarning on <html> silences the expected mismatch.
  const [locale] = useState<Locale>(() => getLocaleFromCookie());

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const localeMessages =
    messages[locale] ?? messages[defaultLocale] ?? {};

  return (
    <NextIntlClientProvider locale={locale} messages={localeMessages} timeZone="UTC">
      {children}
    </NextIntlClientProvider>
  );
}
