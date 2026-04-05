'use client';

import { getDashboard } from '@/lib/api/admin';
import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { StatsCard } from '@/components/stats-card';
import { translateApiError } from '@/lib/i18n/errors';

export default function ModeratorDashboardPage() {
  const t = useTranslations('moderator');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const [data, setData] = useState<Awaited<ReturnType<typeof getDashboard>> | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getDashboard()
      .then(setData)
      .catch((e) => setError(translateApiError(tErr, e)));
  }, [tErr]);

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!data) return <p className="text-muted-foreground">{tCommon('loading')}</p>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">{t('moderatorDashboardTitle')}</h1>
      <div className="mt-6 grid gap-4 sm:grid-cols-3">
        <StatsCard label={t('localUsers')} value={data.local_users_count.toLocaleString()} />
        <StatsCard label={t('localPosts')} value={data.local_statuses_count.toLocaleString()} />
        <StatsCard label={t('openReports')} value={data.open_reports_count.toLocaleString()} />
      </div>
    </div>
  );
}
