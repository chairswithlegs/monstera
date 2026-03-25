import {
  defaultLocale,
  locales,
  LOCALE_COOKIE,
  type Locale,
} from '@/i18n/config';

export function getLocaleFromCookie(): Locale {
  if (typeof document === 'undefined') return defaultLocale;
  const match = document.cookie.match(
    new RegExp(`(?:^|; )${LOCALE_COOKIE}=([^;]*)`)
  );
  if (!match) return defaultLocale;
  const value = decodeURIComponent(match[1]);
  return (locales as readonly string[]).includes(value)
    ? (value as Locale)
    : defaultLocale;
}

export function setLocale(locale: Locale): void {
  const maxAge = 60 * 60 * 24 * 365; // 1 year
  document.cookie = `${LOCALE_COOKIE}=${locale}; path=/; max-age=${maxAge}; SameSite=Lax`;
  window.location.reload();
}
