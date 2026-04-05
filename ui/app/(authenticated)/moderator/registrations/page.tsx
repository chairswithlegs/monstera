'use client';

import {
  getSettings,
  getRegistrations,
  getInvites,
  approveRegistration,
  rejectRegistration,
  createInvite,
  deleteInvite,
  type AdminSettings,
  type PendingRegistration,
  type Invite,
} from '@/lib/api/admin';
import { useEffect, useMemo, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Card, CardContent } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { EmptyState } from '@/components/empty-state';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { translateApiError } from '@/lib/i18n/errors';

type BadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline';

export default function ModeratorRegistrationsPage() {
  const t = useTranslations('moderator');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const tEmpty = useTranslations('empty');
  const [settings, setSettings] = useState<AdminSettings | null>(null);
  const [pending, setPending] = useState<PendingRegistration[]>([]);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);
  const [creating, setCreating] = useState(false);
  const [rejectUserId, setRejectUserId] = useState<string | null>(null);
  const [rejectReason, setRejectReason] = useState('');

  const modeBadge = useMemo<Record<string, { label: string; variant: BadgeVariant }>>(() => ({
    open: { label: t('regModeOpen'), variant: 'secondary' },
    approval: { label: t('regModeApproval'), variant: 'secondary' },
    invite: { label: t('regModeInvite'), variant: 'default' },
    closed: { label: t('regModeClosed'), variant: 'destructive' },
  }), [t]);

  const load = () => {
    Promise.all([
      getSettings().then(setSettings),
      getRegistrations().then((r) => setPending(r.pending)),
      getInvites().then((r) => setInvites(r.invites)),
    ]).catch((e) => setError(translateApiError(tErr, e)));
  };

  useEffect(() => {
    load();
  }, []);

  const approve = async (id: string) => {
    setActing(true);
    try {
      await approveRegistration(id);
      load();
    } catch (e) {
      setError(translateApiError(tErr, e));
    } finally {
      setActing(false);
    }
  };

  const openRejectDialog = (userId: string) => {
    setRejectUserId(userId);
    setRejectReason('');
  };

  const reject = async () => {
    if (!rejectUserId) return;
    setActing(true);
    try {
      await rejectRegistration(rejectUserId, rejectReason);
      setRejectUserId(null);
      load();
    } catch (e) {
      setError(translateApiError(tErr, e));
    } finally {
      setActing(false);
    }
  };

  const create = async () => {
    setCreating(true);
    try {
      await createInvite({});
      load();
    } catch (e) {
      setError(translateApiError(tErr, e));
    } finally {
      setCreating(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await deleteInvite(id);
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

  const mode = settings?.registration_mode ?? 'open';
  const badge = modeBadge[mode] ?? { label: mode, variant: 'secondary' as BadgeVariant };
  const inviteMode = mode === 'invite';

  return (
    <div className="space-y-8">
      <div className="flex items-center gap-3">
        <h1 className="text-2xl font-semibold text-foreground">{t('registrationsTitle')}</h1>
        <Badge variant={badge.variant}>{badge.label}</Badge>
      </div>

      <section>
        <h2 className="mb-4 text-lg font-medium text-foreground">{t('pendingRegistrations')}</h2>
        <Card>
          <CardContent>
            {pending.length === 0 ? (
              <EmptyState message={tEmpty('noPendingRegistrations')} />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('regColUsername')}</TableHead>
                    <TableHead>{t('regColEmail')}</TableHead>
                    <TableHead>{t('regColReason')}</TableHead>
                    <TableHead className="text-right">{t('reportsColActions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pending.map((p) => (
                    <TableRow key={p.user_id}>
                      <TableCell className="font-medium">@{p.username}</TableCell>
                      <TableCell className="text-muted-foreground">{p.email}</TableCell>
                      <TableCell className="text-muted-foreground">{p.registration_reason ?? '—'}</TableCell>
                      <TableCell className="text-right">
                        <Button
                          type="button"
                          variant="default"
                          size="sm"
                          disabled={acting}
                          onClick={() => approve(p.user_id)}
                          className="mr-2"
                        >
                          {tCommon('approve')}
                        </Button>
                        <Button
                          type="button"
                          variant="destructive"
                          size="sm"
                          disabled={acting}
                          onClick={() => openRejectDialog(p.user_id)}
                        >
                          {tCommon('reject')}
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

      <section>
        <div className="mb-4 flex items-center gap-4">
          <h2 className="text-lg font-medium text-foreground">{t('inviteCodes')}</h2>
          <div className="flex items-center gap-2">
            <Button type="button" disabled={creating || !inviteMode} onClick={create}>
              {t('createInvite')}
            </Button>
            {!inviteMode && (
              <span className="text-sm text-muted-foreground">{t('inviteModeRequired')}</span>
            )}
          </div>
        </div>
        <Card>
          <CardContent>
            {invites.length === 0 ? (
              <EmptyState message={tEmpty('noInviteCodes')} />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('inviteColCode')}</TableHead>
                    <TableHead>{t('inviteColUses')}</TableHead>
                    <TableHead>{t('inviteColCreated')}</TableHead>
                    <TableHead>{t('inviteColExpires')}</TableHead>
                    <TableHead className="text-right">{t('reportsColActions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {invites.map((inv) => (
                    <TableRow key={inv.id}>
                      <TableCell className="font-mono">{inv.code}</TableCell>
                      <TableCell className="text-muted-foreground">
                        {inv.uses}
                        {inv.max_uses != null ? ` / ${inv.max_uses}` : ''}
                      </TableCell>
                      <TableCell className="text-muted-foreground">{new Date(inv.created_at).toLocaleString()}</TableCell>
                      <TableCell className="text-muted-foreground">
                        {inv.expires_at ? new Date(inv.expires_at).toLocaleDateString() : t('inviteNeverExpires')}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          type="button"
                          variant="link"
                          size="sm"
                          onClick={() => remove(inv.id)}
                          className="text-destructive"
                        >
                          {tCommon('revoke')}
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

      <Dialog open={rejectUserId !== null} onOpenChange={(open) => !open && setRejectUserId(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('rejectRegistrationTitle')}</DialogTitle>
            <DialogDescription>{t('rejectRegistrationDescription')}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-2 py-4">
            <Label htmlFor="reject-reason">{t('rejectReason')}</Label>
            <Input
              id="reject-reason"
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              placeholder={t('rejectReasonPlaceholder')}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRejectUserId(null)}>
              {tCommon('cancel')}
            </Button>
            <Button variant="destructive" onClick={reject} disabled={acting}>
              {acting ? t('rejecting') : tCommon('reject')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
