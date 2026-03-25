'use client';

import { Suspense, useEffect, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { getPublicAccount } from '@/lib/api/accounts';
import type { PublicAccount } from '@/lib/api/accounts';
import { getConfig } from '@/lib/config';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { translateApiError } from '@/lib/i18n/errors';

function ProfileContent() {
  const searchParams = useSearchParams();
  const username = searchParams.get('u');
  const t = useTranslations('profile');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');

  const [account, setAccount] = useState<PublicAccount | null>(null);
  const [actorURL, setActorURL] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!username) {
      setLoading(false);
      return;
    }
    Promise.all([getPublicAccount(username), getConfig()])
      .then(([acc, config]) => {
        if (acc.acct.includes('@')) {
          setError(t('profileNotFound'));
          return;
        }
        setAccount(acc);
        setActorURL(`${config.server_url}/users/${acc.username}`);
      })
      .catch((e) => setError(translateApiError(tErr, e)))
      .finally(() => setLoading(false));
  }, [username, t, tErr]);

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <p className="text-muted-foreground">{tCommon('loading')}</p>
      </div>
    );
  }

  if (!username) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Alert variant="destructive" className="max-w-md">
          <AlertDescription>{t('noProfileSpecified')}</AlertDescription>
        </Alert>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Alert variant="destructive" className="max-w-md">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      </div>
    );
  }

  if (!account) return null;

  const displayName = account.display_name || account.username;
  const hasHeader = !!account.header;

  return (
    <div className="max-w-2xl mx-auto py-8 px-4">
      <Card className="overflow-hidden">
        {/* Header / banner */}
        <div className="relative h-40 bg-muted">
          {hasHeader && (
            <img
              src={account.header}
              alt=""
              className="w-full h-full object-cover"
            />
          )}
          {/* Avatar */}
          {account.avatar && (
            <div className="absolute -bottom-10 left-6">
              <img
                src={account.avatar}
                alt={displayName}
                className="w-20 h-20 rounded-full border-4 border-background object-cover"
              />
            </div>
          )}
        </div>

        <CardContent className={`${account.avatar ? 'pt-14' : 'pt-6'} pb-6`}>
          {/* Name + handle */}
          <div className="flex items-start justify-between gap-2">
            <div>
              <h1 className="text-xl font-bold">{displayName}</h1>
              <p className="text-sm text-muted-foreground">@{account.acct}</p>
            </div>
            <div className="flex gap-1.5 flex-shrink-0">
              {account.bot && <Badge variant="secondary">{t('botBadge')}</Badge>}
              {account.locked && <Badge variant="outline">{t('lockedBadge')}</Badge>}
            </div>
          </div>

          {/* Bio */}
          {account.note && (
            <div
              className="mt-4 text-sm prose prose-sm max-w-none"
              dangerouslySetInnerHTML={{ __html: account.note }}
            />
          )}

          {/* Stats */}
          <div className="mt-4 flex gap-6 text-sm">
            <div>
              <span className="font-semibold">{account.statuses_count}</span>{' '}
              <span className="text-muted-foreground">{t('posts')}</span>
            </div>
            <div>
              <span className="font-semibold">{account.following_count}</span>{' '}
              <span className="text-muted-foreground">{t('following')}</span>
            </div>
            <div>
              <span className="font-semibold">{account.followers_count}</span>{' '}
              <span className="text-muted-foreground">{t('followers')}</span>
            </div>
          </div>

          {/* Profile fields */}
          {account.fields.length > 0 && (
            <div className="mt-4 border rounded-md divide-y text-sm">
              {account.fields.map((f, i) => (
                <div key={i} className="flex px-3 py-2 gap-4">
                  <span className="font-medium text-muted-foreground w-28 flex-shrink-0">
                    {f.name}
                  </span>
                  <span
                    className="break-all"
                    dangerouslySetInnerHTML={{ __html: f.value }}
                  />
                </div>
              ))}
            </div>
          )}

          {/* ActivityPub ID */}
          {actorURL && (
            <div className="mt-4 text-xs text-muted-foreground">
              <span>{t('activityPubId')}</span>
              <a
                href={actorURL}
                className="underline underline-offset-2 hover:text-foreground break-all"
              >
                {actorURL}
              </a>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function LoadingFallback() {
  const tCommon = useTranslations('common');
  return (
    <div className="flex items-center justify-center min-h-screen">
      <p className="text-muted-foreground">{tCommon('loading')}</p>
    </div>
  );
}

export default function ProfilePage() {
  return (
    <Suspense fallback={<LoadingFallback />}>
      <ProfileContent />
    </Suspense>
  );
}
