'use client';

import { getUser } from '@/lib/api/user';
import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import type { User } from '@/lib/api/user';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { translateApiError } from '@/lib/i18n/errors';

export default function HomePage() {
  const t = useTranslations('home');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getUser()
      .then(setUser)
      .catch((e) => setError(translateApiError(tErr, e)))
      .finally(() => setLoading(false));
  }, [tErr]);

  if (loading) return <div className="text-muted-foreground">{tCommon('loading')}</div>;
  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!user) return <div className="text-muted-foreground">{tCommon('loading')}</div>;

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-semibold">{t('welcome', { username: user.username })}</h1>

      <section className="space-y-2">
        <h2 className="text-lg font-medium">{t('whatIsMonstera')}</h2>
        <p className="text-muted-foreground">
          {t('whatIsMonsteraBody')}
        </p>
      </section>

      <section className="space-y-2">
        <h2 className="text-lg font-medium">{t('whatIsThisUI')}</h2>
        <p className="text-muted-foreground">
          {t('whatIsThisUIBody')}
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-medium">{t('recommendedClients')}</h2>
        <ul className="space-y-2">
          <li>
            <a
              href="https://elk.zone"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
            >
              Elk
            </a>
            <span className="text-muted-foreground"> {t('elkDescription')}</span>
          </li>
          <li>
            <a
              href="https://mastodon.social/about"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
            >
              Mastodon
            </a>
            <span className="text-muted-foreground"> {t('mastodonDescription')}</span>
          </li>
          <li>
            <a
              href="https://tapbots.com/ivory"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
            >
              Ivory
            </a>
            <span className="text-muted-foreground"> {t('ivoryDescription')}</span>
          </li>
          <li>
            <a
              href="https://getmammoth.app"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
            >
              Mammoth
            </a>
            <span className="text-muted-foreground"> {t('mammothDescription')}</span>
          </li>
          <li>
            <a
              href="https://tusky.app"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
            >
              Tusky
            </a>
            <span className="text-muted-foreground"> {t('tuskyDescription')}</span>
          </li>
        </ul>
      </section>
    </div>
  );
}
