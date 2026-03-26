'use client';

import { getReports, type Report } from '@/lib/api/admin';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import ReportDetailView from './ReportDetailView';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
} from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { EmptyState } from '@/components/empty-state';
import { translateApiError } from '@/lib/i18n/errors';

export default function ModeratorReportsPage() {
  const searchParams = useSearchParams();
  const detailId = searchParams.get('id');

  if (detailId) {
    return <ReportDetailView id={detailId} />;
  }

  return <ReportsList />;
}

type ReportState = 'open' | 'resolved' | 'all';

function ReportsList() {
  const t = useTranslations('moderator');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const tEmpty = useTranslations('empty');
  const [reports, setReports] = useState<Report[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [state, setState] = useState<ReportState>('open');

  const apiState = state === 'all' ? '' : state;

  useEffect(() => {
    getReports({ state: apiState, limit: 50 })
      .then((r) => setReports(r.reports))
      .catch((e) => setError(translateApiError(tErr, e)));
  }, [apiState, tErr]);

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">{t('reportsTitle')}</h1>
      <Tabs value={state} onValueChange={(v) => setState(v as ReportState)} className="mt-4">
        <TabsList>
          <TabsTrigger value="open">{t('reportsTabOpen')}</TabsTrigger>
          <TabsTrigger value="resolved">{t('reportsTabResolved')}</TabsTrigger>
          <TabsTrigger value="all">{t('reportsTabAll')}</TabsTrigger>
        </TabsList>
      </Tabs>
      <Card className="mt-6">
        <CardContent>
              {reports.length === 0 ? (
                <EmptyState message={tEmpty('noReports')} />
              ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('reportsColId')}</TableHead>
                  <TableHead>{t('reportsColCategory')}</TableHead>
                  <TableHead>{t('reportsColState')}</TableHead>
                  <TableHead>{t('reportsColCreated')}</TableHead>
                  <TableHead className="text-right">{t('reportsColActions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {reports.map((r) => (
                  <TableRow key={r.id}>
                    <TableCell className="font-mono">{r.id.slice(0, 8)}…</TableCell>
                    <TableCell className="text-muted-foreground">{r.category}</TableCell>
                    <TableCell>
                      <Badge variant={r.state === 'open' ? 'secondary' : 'outline'}>
                        {r.state}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{new Date(r.created_at).toLocaleString()}</TableCell>
                    <TableCell className="text-right">
                      <Button variant="link" size="sm" asChild>
                        <Link href={`/moderator/reports?id=${encodeURIComponent(r.id)}`}>
                          {tCommon('view')}
                        </Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
