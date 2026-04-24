'use client';

import { getInstances, getDomainBlocks, createDomainBlock, deleteDomainBlock, type KnownInstance, type DomainBlock } from '@/lib/api/admin';
import { useModeratorUser } from '@/contexts/moderator-user';
import { isAdmin } from '@/lib/api/user';
import { useCallback, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
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
import { Badge } from '@/components/ui/badge';

export default function ModeratorFederationPage() {
  const currentUser = useModeratorUser();
  const t = useTranslations('moderator');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const tEmpty = useTranslations('empty');
  const [instances, setInstances] = useState<KnownInstance[]>([]);
  const [blocks, setBlocks] = useState<DomainBlock[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [newDomain, setNewDomain] = useState('');
  const [newSeverity, setNewSeverity] = useState<'silence' | 'suspend'>('silence');
  const [newReason, setNewReason] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(() => {
    getInstances({ limit: 50 })
      .then((r) => setInstances(r.instances))
      .catch((e) => setError(translateApiError(tErr, e)));
    getDomainBlocks()
      .then((r) => setBlocks(r.domain_blocks))
      .catch((e) => setError(translateApiError(tErr, e)));
  }, [tErr]);

  useEffect(() => {
    load();
  }, [load]);

  // Poll every 5s while at least one block has an in-progress purge so the
  // "Purging… N remaining" badge ticks down without a manual refresh.
  // Disabled otherwise to avoid extra load — the admin list is otherwise
  // static between mutations.
  const hasInProgressPurge = blocks.some((b) => b.purge_status === 'in_progress');
  useEffect(() => {
    if (!hasInProgressPurge) return;
    const id = window.setInterval(load, 5000);
    return () => window.clearInterval(id);
  }, [hasInProgressPurge, load]);

  const isDuplicate = blocks.some((b) => b.domain.toLowerCase() === newDomain.trim().toLowerCase());

  const addBlock = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDomain.trim() || isDuplicate) return;
    setSubmitting(true);
    try {
      await createDomainBlock({ domain: newDomain.trim(), severity: newSeverity, reason: newReason.trim() || undefined });
      setNewDomain('');
      setNewSeverity('silence');
      setNewReason('');
      load();
    } catch (e) {
      setError(translateApiError(tErr, e));
    } finally {
      setSubmitting(false);
    }
  };

  const removeBlock = async (domain: string) => {
    try {
      await deleteDomainBlock(domain);
      load();
    } catch (e) {
      setError(translateApiError(tErr, e));
    }
  };

  const showAdminActions = currentUser && isAdmin(currentUser);

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">{t('federationTitle')}</h1>

      {error && (
        <Alert variant="destructive" className="mt-4">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <section className="mt-8">
        <h2 className="text-lg font-medium text-foreground">{t('knownInstances')}</h2>
        <Card className="mt-4">
          <CardContent>
            {instances.length === 0 ? (
              <EmptyState message={tEmpty('noInstances')} />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('domainColDomain')}</TableHead>
                    <TableHead>{t('domainColAccounts')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {instances.map((i) => (
                    <TableRow key={i.id}>
                      <TableCell>{i.domain}</TableCell>
                      <TableCell className="text-muted-foreground">{i.accounts_count}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </section>

      <section className="mt-8">
        <h2 className="text-lg font-medium text-foreground">{t('domainBlocks')}</h2>
        {showAdminActions && (
          <form onSubmit={addBlock} className="mt-4 flex flex-wrap items-end gap-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="domain-block-domain">{t('domainBlockDomain')}</Label>
              <Input
                id="domain-block-domain"
                type="text"
                value={newDomain}
                onChange={(e) => setNewDomain(e.target.value)}
                placeholder="domain.example.com"
                className="min-w-[200px]"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="domain-block-severity">{t('domainBlockSeverity')}</Label>
              <select
                id="domain-block-severity"
                value={newSeverity}
                onChange={(e) => setNewSeverity(e.target.value as 'silence' | 'suspend')}
                className="h-9 rounded-md border border-input bg-transparent px-3 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                <option value="silence">{t('domainBlockSeveritySilence')}</option>
                <option value="suspend">{t('domainBlockSeveritySuspend')}</option>
              </select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="domain-block-reason">{t('domainBlockReason')}</Label>
              <Input
                id="domain-block-reason"
                type="text"
                value={newReason}
                onChange={(e) => setNewReason(e.target.value)}
                placeholder={t('domainBlockReason')}
                className="min-w-[200px]"
              />
            </div>
            <Button type="submit" disabled={submitting || isDuplicate}>
              {t('addBlock')}
            </Button>
            {isDuplicate && (
              <p className="text-sm text-destructive">{t('domainBlockDuplicate')}</p>
            )}
          </form>
        )}
        <Card className="mt-4">
          <CardContent>
            {blocks.length === 0 ? (
              <EmptyState message={tEmpty('noDomainBlocks')} />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('domainColDomain')}</TableHead>
                    <TableHead>{t('domainColSeverity')}</TableHead>
                    <TableHead>{t('domainColReason')}</TableHead>
                    <TableHead>{t('domainColPurge')}</TableHead>
                    {showAdminActions && <TableHead className="text-right">{t('domainColActions')}</TableHead>}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {blocks.map((b) => (
                    <TableRow key={b.id}>
                      <TableCell>{b.domain}</TableCell>
                      <TableCell className="text-muted-foreground">{b.severity}</TableCell>
                      <TableCell className="text-muted-foreground">{b.reason}</TableCell>
                      <TableCell>
                        {b.purge_status === 'in_progress' && (
                          typeof b.purge_accounts_remaining === 'number' ? (
                            <Badge variant="secondary">
                              {t('purgeInProgressWithCount', { remaining: b.purge_accounts_remaining })}
                            </Badge>
                          ) : (
                            <Badge variant="secondary">{t('purgeInProgress')}</Badge>
                          )
                        )}
                        {b.purge_status === 'complete' && (
                          <Badge variant="outline" title={b.purge_completed_at ? t('purgeCompleteTooltip', { completedAt: b.purge_completed_at }) : undefined}>
                            {t('purgeComplete')}
                          </Badge>
                        )}
                        {!b.purge_status && <span className="text-muted-foreground">—</span>}
                      </TableCell>
                      {showAdminActions && (
                        <TableCell className="text-right">
                          <Button type="button" variant="link" size="sm" onClick={() => removeBlock(b.domain)} className="text-destructive">
                            {tCommon('remove')}
                          </Button>
                        </TableCell>
                      )}
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
