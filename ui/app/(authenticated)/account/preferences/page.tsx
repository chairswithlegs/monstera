'use client';

import { getUser, patchPreferences } from '@/lib/api/user';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { locales, localeNames, defaultLocale, type Locale } from '@/i18n/config';
import { getLocaleFromCookie, setLocale } from '@/lib/i18n/locale';
import { translateApiError } from '@/lib/i18n/errors';

// Common BCP-47 language tags shown in the post language dropdown.
// Covers the most widely used languages on the fediverse.
const LANGUAGES = [
  'ar', 'ca', 'cs', 'da', 'de', 'el', 'en', 'eo', 'es', 'eu',
  'fi', 'fr', 'ga', 'gl', 'he', 'hu', 'id', 'it', 'ja', 'ka',
  'ko', 'lt', 'nl', 'no', 'oc', 'pl', 'pt', 'pt-BR', 'ru', 'sk',
  'sl', 'sq', 'sr', 'sv', 'ta', 'th', 'tr', 'uk', 'zh', 'zh-Hant',
];

const visibilityOptions = [
  { value: 'public', key: 'visibilityPublic' },
  { value: 'unlisted', key: 'visibilityUnlisted' },
  { value: 'private', key: 'visibilityPrivate' },
  { value: 'direct', key: 'visibilityDirect' },
] as const;

const quotePolicyOptions = [
  { value: 'public', key: 'quotePolicyAnyone' },
  { value: 'followers', key: 'quotePolicyFollowers' },
  { value: 'nobody', key: 'quotePolicyNobody' },
] as const;

export default function PreferencesPage() {
  const t = useTranslations('account');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const [defaultPrivacy, setDefaultPrivacy] = useState('public');
  const [defaultSensitive, setDefaultSensitive] = useState(false);
  const [defaultLanguage, setDefaultLanguage] = useState('');
  const [defaultQuotePolicy, setDefaultQuotePolicy] = useState('public');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [uiLocale, setUiLocale] = useState<Locale>(defaultLocale);

  // Include current value in options if it's not in the standard list (avoids data loss).
  const langOptions = useMemo(() => {
    const inList = defaultLanguage === '' || LANGUAGES.includes(defaultLanguage);
    return inList ? LANGUAGES : [...LANGUAGES, defaultLanguage].sort();
  }, [defaultLanguage]);

  const langNames = useMemo(
    () => new Intl.DisplayNames([uiLocale], { type: 'language' }),
    [uiLocale]
  );

  const load = useCallback(() => {
    getUser()
      .then((u) => {
        setDefaultPrivacy(u.default_privacy || 'public');
        setDefaultSensitive(u.default_sensitive);
        setDefaultLanguage(u.default_language || '');
        setDefaultQuotePolicy(u.default_quote_policy || 'public');
      })
      .catch((e) => setError(translateApiError(tErr, e)))
      .finally(() => setLoading(false));
  }, [tErr]);

  useEffect(() => {
    load();
    setUiLocale(getLocaleFromCookie());
  }, [load]);

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      await patchPreferences({
        default_privacy: defaultPrivacy,
        default_sensitive: defaultSensitive,
        default_language: defaultLanguage,
        default_quote_policy: defaultQuotePolicy,
      });
      setSuccess(true);
    } catch (e) {
      setError(translateApiError(tErr, e));
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="text-muted-foreground">{t('preferencesTitle')}…</div>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">{t('preferencesTitle')}</h1>
      <p className="mt-2 text-gray-500">{t('preferencesDescription')}</p>
      {error && (
        <Alert variant="destructive" className="mt-4">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {success && (
        <Alert variant="default" className="mt-4">
          <AlertDescription>{t('preferencesSaved')}</AlertDescription>
        </Alert>
      )}
      <form onSubmit={save} className="mt-6 max-w-2xl space-y-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="default-privacy">{t('defaultPostVisibility')}</Label>
          <p className="text-xs text-muted-foreground">{t('defaultPostVisibilityHint')}</p>
          <select
            id="default-privacy"
            value={defaultPrivacy}
            onChange={(e) => setDefaultPrivacy(e.target.value)}
            className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            {visibilityOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {t(opt.key)}
              </option>
            ))}
          </select>
        </div>
        <div className="flex flex-col gap-3">
          <div>
            <h2 className="text-base font-medium text-foreground">{t('language')}</h2>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="default-language">{t('postLanguage')}</Label>
            <p className="text-xs text-muted-foreground">{t('defaultPostLanguageHint')}</p>
            <select
              id="default-language"
              value={defaultLanguage}
              onChange={(e) => setDefaultLanguage(e.target.value)}
              className="flex h-9 w-64 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            >
              <option value="">{t('postLanguageNotSet')}</option>
              {langOptions.map((code) => (
                <option key={code} value={code}>
                  {langNames.of(code) ?? code}
                </option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="ui-locale">{t('interfaceLanguage')}</Label>
            <p className="text-xs text-muted-foreground">{t('languageHint')}</p>
            <select
              id="ui-locale"
              value={uiLocale}
              onChange={(e) => setLocale(e.target.value as Locale)}
              className="flex h-9 w-64 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            >
              {locales.map((loc) => (
                <option key={loc} value={loc}>
                  {localeNames[loc]}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center gap-2">
            <input
              id="default-sensitive"
              type="checkbox"
              checked={defaultSensitive}
              onChange={(e) => setDefaultSensitive(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300"
            />
            <Label htmlFor="default-sensitive">{t('markSensitive')}</Label>
          </div>
          <p className="text-xs text-muted-foreground">{t('markSensitiveHint')}</p>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="default-quote-policy">{t('quotePolicy')}</Label>
          <p className="text-xs text-muted-foreground">{t('quotePolicyHint')}</p>
          <select
            id="default-quote-policy"
            value={defaultQuotePolicy}
            onChange={(e) => setDefaultQuotePolicy(e.target.value)}
            className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            {quotePolicyOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {t(opt.key)}
              </option>
            ))}
          </select>
        </div>
        <Button type="submit" disabled={saving}>
          {saving ? tCommon('saving') : tCommon('save')}
        </Button>
      </form>
    </div>
  );
}
