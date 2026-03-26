'use client';

import { getFilters, createFilter, deleteFilter, type ServerFilter } from '@/lib/api/admin';
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
  const [filters, setFilters] = useState<ServerFilter[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [newPhrase, setNewPhrase] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(() => {
    getFilters()
      .then((r) => setFilters(r.filters))
      .catch((e) => setError(translateApiError(tErr, e)));
  }, [tErr]);

  useEffect(() => {
    load();
  }, [load]);

  const addFilter = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newPhrase.trim()) return;
    setSubmitting(true);
    try {
      await createFilter({ phrase: newPhrase.trim(), scope: 'all', action: 'hide' });
      setNewPhrase('');
      load();
    } catch (e) {
      setError(translateApiError(tErr, e));
    } finally {
      setSubmitting(false);
    }
  };

  const removeFilter = async (id: string) => {
    try {
      await deleteFilter(id);
      load();
    } catch (e) {
      setError(translateApiError(tErr, e));
    }
  };

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">{t('contentTitle')}</h1>
      <p className="mt-1 text-sm text-muted-foreground">{t('contentDescription')}</p>

      <section className="mt-8">
        <h2 className="text-lg font-medium text-foreground">{t('serverFilters')}</h2>
        <form onSubmit={addFilter} className="mt-4 flex gap-2">
          <Input
            type="text"
            value={newPhrase}
            onChange={(e) => setNewPhrase(e.target.value)}
            placeholder={t('filterKeyword')}
            className="min-w-[200px]"
          />
          <Button type="submit" disabled={submitting}>
            {t('addFilter')}
          </Button>
        </form>
        <Card className="mt-4">
          <CardContent>
            {filters.length === 0 ? (
              <EmptyState message={tEmpty('noFilters')} />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('filterColPhrase')}</TableHead>
                    <TableHead>{t('filterColScope')}</TableHead>
                    <TableHead>{t('filterColAction')}</TableHead>
                    <TableHead className="text-right">{t('filterColActions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filters.map((f) => (
                    <TableRow key={f.id}>
                      <TableCell>{f.phrase}</TableCell>
                      <TableCell className="text-muted-foreground">{f.scope}</TableCell>
                      <TableCell className="text-muted-foreground">{f.action}</TableCell>
                      <TableCell className="text-right">
                        <Button type="button" variant="ghost" size="sm" onClick={() => removeFilter(f.id)} className="text-destructive hover:text-destructive">
                          {tCommon('delete')}
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
