'use client';

import {
  getTrendingLinkDenylist,
  addTrendingLinkDenylist,
  removeTrendingLinkDenylist,
} from '@/lib/api/admin';
import { getUser, isAdmin } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { translateApiError } from '@/lib/i18n/errors';

export default function AdminTrendsPage() {
  const router = useRouter();
  const t = useTranslations('admin');
  const tCommon = useTranslations('common');
  const tEmpty = useTranslations('empty');
  const tErr = useTranslations('errors');

  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const [denylist, setDenylist] = useState<string[]>([]);
  const [denylistUrl, setDenylistUrl] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getUser()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!user || !isAdmin(user)) {
      router.replace('/home');
    }
  }, [loading, user, router]);

  const loadDenylist = useCallback(() => {
    getTrendingLinkDenylist()
      .then((d) => setDenylist(d.urls))
      .catch((e) => setError(translateApiError(tErr, e)));
  }, [tErr]);

  useEffect(() => {
    if (user && isAdmin(user)) loadDenylist();
  }, [user, loadDenylist]);

  const handleAddDenylist = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      await addTrendingLinkDenylist(denylistUrl.trim());
      setDenylistUrl('');
      loadDenylist();
    } catch (e) {
      setError(translateApiError(tErr, e));
    }
  };

  const handleRemoveDenylist = async (url: string) => {
    setError(null);
    try {
      await removeTrendingLinkDenylist(url);
      loadDenylist();
    } catch (e) {
      setError(translateApiError(tErr, e));
    }
  };

  if (loading || !user || !isAdmin(user)) {
    return null;
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">{t('trendsTitle')}</h1>
      <p className="mt-2 text-gray-500">{t('trendsDescription')}</p>

      {error && (
        <Alert variant="destructive" className="mt-4">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="mt-8 max-w-2xl">
        <section>
          <h2 className="text-lg font-medium text-gray-900">{t('denylistTitle')}</h2>
          <p className="mt-1 text-sm text-gray-500">{t('denylistDescription')}</p>
          <form onSubmit={handleAddDenylist} className="mt-3 flex gap-2">
            <Input
              type="url"
              placeholder={t('urlPlaceholder')}
              value={denylistUrl}
              onChange={(e) => setDenylistUrl(e.target.value)}
              className="flex-1"
            />
            <Button type="submit" disabled={!denylistUrl.trim()}>
              {t('addUrl')}
            </Button>
          </form>
          <ul className="mt-4 divide-y divide-gray-100 rounded-md border border-gray-200">
            {denylist.length === 0 ? (
              <li className="px-4 py-3 text-sm text-gray-500">{tEmpty('noDenylistedLinks')}</li>
            ) : (
              denylist.map((url) => (
                <li key={url} className="flex items-center justify-between px-4 py-2 text-sm">
                  <span className="truncate text-gray-700">{url}</span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleRemoveDenylist(url)}
                  >
                    {tCommon('remove')}
                  </Button>
                </li>
              ))
            )}
          </ul>
        </section>
      </div>
    </div>
  );
}
