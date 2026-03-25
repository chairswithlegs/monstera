'use client';

import {
  getUser,
  suspendUser,
  unsuspendUser,
  silenceUser,
  unsilenceUser,
  type AdminUser,
} from '@/lib/api/admin';
import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { translateApiError } from '@/lib/i18n/errors';

type Props = { id: string };

export default function UserDetailView({ id }: Props) {
  const t = useTranslations('moderator');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const [user, setUser] = useState<AdminUser | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);

  const load = useCallback(() => {
    if (!id) return;
    getUser(id)
      .then(setUser)
      .catch((e) => setError(translateApiError(tErr, e)));
  }, [id, tErr]);

  useEffect(() => {
    load();
  }, [load]);

  const act = async (fn: () => Promise<void>) => {
    setActing(true);
    try {
      await fn();
      load();
    } catch (e) {
      setError(translateApiError(tErr, e));
    } finally {
      setActing(false);
    }
  };

  if (error && !user) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!user) return <p className="text-muted-foreground">{tCommon('loading')}</p>;

  const isAdmin = user.role === 'admin';

  return (
    <div>
      <p className="mb-4">
        <Button variant="link" size="sm" className="h-auto p-0" asChild>
          <Link href="/moderator/users">{t('backToUsers')}</Link>
        </Button>
      </p>
      <h1 className="text-2xl font-semibold text-foreground">@{user.username}</h1>
      <p className="mt-1 text-muted-foreground">{user.email}</p>
      <p className="mt-1 text-sm text-muted-foreground">{t('accountId', { id: user.account_id })}</p>
      <p className="mt-1 text-sm text-muted-foreground">{t('role', { role: user.role })}</p>
      {user.suspended && <p className="mt-2 text-destructive">{t('badgeSuspended')}</p>}
      {user.silenced && !user.suspended && <p className="mt-2 text-amber-600 dark:text-amber-500">{t('badgeSilenced')}</p>}
      {error && (
        <Alert variant="destructive" className="mt-2">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <div className="mt-6 flex flex-wrap gap-2">
        {user.suspended ? (
          <Button type="button" disabled={acting || isAdmin} onClick={() => act(() => unsuspendUser(user.account_id))}>
            {t('unsuspendUser')}
          </Button>
        ) : (
          <Button type="button" variant="destructive" disabled={acting || isAdmin} onClick={() => act(() => suspendUser(user.account_id))}>
            {t('suspendUser')}
          </Button>
        )}
        {user.silenced ? (
          <Button type="button" disabled={acting || isAdmin} onClick={() => act(() => unsilenceUser(user.account_id))}>
            {t('unsilenceUser')}
          </Button>
        ) : (
          <Button type="button" variant="secondary" disabled={acting || isAdmin} onClick={() => act(() => silenceUser(user.account_id))} className="bg-amber-600 text-white hover:bg-amber-700 hover:text-white">
            {t('silenceUser')}
          </Button>
        )}
      </div>
      {isAdmin && (
        <p className="mt-4 text-sm text-muted-foreground">{t('adminActionsDisabled')}</p>
      )}
    </div>
  );
}
