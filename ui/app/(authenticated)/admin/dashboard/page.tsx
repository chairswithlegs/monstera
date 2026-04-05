'use client';

import { getAdminMetrics, type AdminMetricsResponse } from '@/lib/api/admin';
import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { StatsCard } from '@/components/stats-card';
import { translateApiError } from '@/lib/i18n/errors';

export default function AdminMetricsPage() {
  const t = useTranslations('admin');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const [data, setData] = useState<AdminMetricsResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getAdminMetrics()
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
      <h1 className="text-2xl font-semibold text-foreground">{t('adminDashboardTitle')}</h1>

      <h2 className="mt-6 text-lg font-medium text-foreground">{t('metricsInstanceActivity')}</h2>
      <div className="mt-3 grid gap-4 sm:grid-cols-3">
        <StatsCard label={t('localAccounts')} value={data.local_accounts.toLocaleString()} />
        <StatsCard label={t('remoteAccounts')} value={data.remote_accounts.toLocaleString()} />
        <StatsCard label={t('localStatuses')} value={data.local_statuses.toLocaleString()} />
        <StatsCard label={t('remoteStatuses')} value={data.remote_statuses.toLocaleString()} />
        <StatsCard label={t('knownInstances')} value={data.known_instances.toLocaleString()} />
      </div>

      <h2 className="mt-6 text-lg font-medium text-foreground">{t('metricsModeration')}</h2>
      <div className="mt-3 grid gap-4 sm:grid-cols-3">
        <StatsCard label={t('openReports')} value={data.open_reports.toLocaleString()} />
      </div>

      <h2 className="mt-6 text-lg font-medium text-foreground">{t('metricsFederation')}</h2>
      <div className="mt-3 grid gap-4 sm:grid-cols-3">
        <StatsCard label={t('deliveryDlqDepth')} value={data.delivery_dlq_depth.toLocaleString()} />
        <StatsCard label={t('fanoutDlqDepth')} value={data.fanout_dlq_depth.toLocaleString()} />
      </div>
    </div>
  );
}
