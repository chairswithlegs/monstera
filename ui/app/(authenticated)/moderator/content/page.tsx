'use client';

import { getTrendingLinkFilters, addTrendingLinkFilter, removeTrendingLinkFilter } from '@/lib/api/admin';
import { useCallback, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Alert, AlertDescription } from '@/components/ui/alert';
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
import { EmptyState } from '@/components/empty-state';
import { translateApiError } from '@/lib/i18n/errors';

export default function ModeratorContentPage() {
  const t = useTranslations('moderator');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const tEmpty = useTranslations('empty');
  const [error, setError] = useState<string | null>(null);

  const [trendingLinkFilterUrls, setTrendingLinkFilterUrls] = useState<string[]>([]);
  const [newTrendingLinkUrl, setNewTrendingLinkUrl] = useState('');

  const loadTrendingLinkFilters = useCallback(() => {
    getTrendingLinkFilters()
      .then((d) => setTrendingLinkFilterUrls(d.urls))
      .catch((e) => setError(translateApiError(tErr, e)));
  }, [tErr]);

  useEffect(() => {
    loadTrendingLinkFilters();
  }, [loadTrendingLinkFilters]);

  const handleAddTrendingLinkFilter = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTrendingLinkUrl.trim()) return;
    setError(null);
    try {
      await addTrendingLinkFilter(newTrendingLinkUrl.trim());
      setNewTrendingLinkUrl('');
      loadTrendingLinkFilters();
    } catch (e) {
      setError(translateApiError(tErr, e));
    }
  };

  const handleRemoveTrendingLinkFilter = async (url: string) => {
    setError(null);
    try {
      await removeTrendingLinkFilter(url);
      loadTrendingLinkFilters();
    } catch (e) {
      setError(translateApiError(tErr, e));
    }
  };

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">{t('contentTitle')}</h1>
      <p className="mt-1 text-sm text-muted-foreground">{t('contentDescription')}</p>

      {error && (
        <Alert variant="destructive" className="mt-4">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <section className="mt-8">
        <h2 className="text-lg font-medium text-foreground">{t('trendingLinkFilterTitle')}</h2>
        <p className="mt-1 text-sm text-muted-foreground">{t('trendingLinkFilterDescription')}</p>
        <form onSubmit={handleAddTrendingLinkFilter} className="mt-4 flex gap-2">
          <Input
            type="url"
            value={newTrendingLinkUrl}
            onChange={(e) => setNewTrendingLinkUrl(e.target.value)}
            placeholder={t('trendingLinkFilterPlaceholder')}
            className="flex-1"
          />
          <Button type="submit" disabled={!newTrendingLinkUrl.trim()}>
            {t('trendingLinkFilterAdd')}
          </Button>
        </form>
        <Card className="mt-4">
          <CardContent>
            {trendingLinkFilterUrls.length === 0 ? (
              <EmptyState message={tEmpty('noTrendingLinkFilters')} />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('trendingLinkFilterColUrl')}</TableHead>
                    <TableHead className="text-right">{t('trendingLinkFilterColActions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {trendingLinkFilterUrls.map((url) => (
                    <TableRow key={url}>
                      <TableCell className="truncate">{url}</TableCell>
                      <TableCell className="text-right">
                        <Button type="button" variant="link" size="sm" onClick={() => handleRemoveTrendingLinkFilter(url)} className="text-destructive">
                          {tCommon('remove')}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
