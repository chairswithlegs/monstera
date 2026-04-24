'use client';

import { deleteAccount, getUser, patchEmail, patchPassword } from '@/lib/api/user';
import {
  AuthorizedApp,
  listAuthorizedApps,
  revokeAuthorizedApp,
  verifyAppCredentials,
  VerifiedAppCredentials,
} from '@/lib/api/oauth-apps';
import { useCallback, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { EmptyState } from '@/components/empty-state';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { translateApiError } from '@/lib/i18n/errors';
import { logout } from '@/lib/auth/logout';

export default function SecurityPage() {
  const t = useTranslations('account');
  const tApps = useTranslations('authorizedApps');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const [currentEmail, setCurrentEmail] = useState('');
  const [email, setEmail] = useState('');
  const [emailError, setEmailError] = useState<string | null>(null);
  const [emailSuccess, setEmailSuccess] = useState(false);
  const [savingEmail, setSavingEmail] = useState(false);

  const loadUser = useCallback(() => {
    getUser()
      .then((u) => setCurrentEmail(u.email))
      .catch(() => {});
  }, []);

  useEffect(() => {
    loadUser();
  }, [loadUser]);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [passwordSuccess, setPasswordSuccess] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deletePassword, setDeletePassword] = useState('');
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [apps, setApps] = useState<AuthorizedApp[] | null>(null);
  const [currentApp, setCurrentApp] = useState<VerifiedAppCredentials | null>(null);
  const [appsError, setAppsError] = useState<string | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<AuthorizedApp | null>(null);
  const [revokeError, setRevokeError] = useState<string | null>(null);
  const [revoking, setRevoking] = useState(false);

  useEffect(() => {
    let cancelled = false;
    Promise.all([listAuthorizedApps(), verifyAppCredentials().catch(() => null)])
      .then(([list, current]) => {
        if (cancelled) return;
        setApps(list);
        setCurrentApp(current);
        setAppsError(null);
      })
      .catch((err) => {
        if (cancelled) return;
        setAppsError(translateApiError(tErr, err));
        setApps([]);
      });
    return () => {
      cancelled = true;
    };
  }, [tErr]);

  const isCurrentSession = (app: AuthorizedApp): boolean =>
    currentApp !== null && currentApp.id === app.id;

  const openRevoke = (app: AuthorizedApp | null) => {
    setRevokeTarget(app);
    setRevokeError(null);
    setRevoking(false);
  };

  const submitRevoke = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!revokeTarget) return;
    setRevoking(true);
    setRevokeError(null);
    try {
      await revokeAuthorizedApp(revokeTarget.id);
      if (isCurrentSession(revokeTarget)) {
        logout();
        return;
      }
      setApps((prev) => (prev ? prev.filter((a) => a.id !== revokeTarget.id) : prev));
      openRevoke(null);
    } catch (err) {
      setRevokeError(translateApiError(tErr, err));
      setRevoking(false);
    }
  };

  const formatDate = (iso: string): string => {
    try {
      return new Date(iso).toLocaleDateString();
    } catch {
      return iso;
    }
  };

  const saveEmail = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingEmail(true);
    setEmailError(null);
    setEmailSuccess(false);
    try {
      const updated = await patchEmail({ email });
      setCurrentEmail(updated.email);
      setEmailSuccess(true);
      setEmail('');
    } catch (e) {
      setEmailError(translateApiError(tErr, e));
    } finally {
      setSavingEmail(false);
    }
  };

  const savePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingPassword(true);
    setPasswordError(null);
    setPasswordSuccess(false);
    try {
      await patchPassword({ current_password: currentPassword, new_password: newPassword });
      setPasswordSuccess(true);
      setCurrentPassword('');
      setNewPassword('');
    } catch (e) {
      setPasswordError(translateApiError(tErr, e));
    } finally {
      setSavingPassword(false);
    }
  };

  const submitDelete = async (e: React.FormEvent) => {
    e.preventDefault();
    setDeleting(true);
    setDeleteError(null);
    try {
      await deleteAccount({ current_password: deletePassword });
      // Backend hard-deleted the account; CASCADE drops every OAuth token. Clear
      // local session and hard-redirect to /login so any stale token resolves
      // to 401 on the next request.
      logout();
    } catch (err) {
      setDeleteError(translateApiError(tErr, err));
      setDeleting(false);
    }
  };

  const openDelete = (open: boolean) => {
    setDeleteOpen(open);
    if (!open) {
      setDeletePassword('');
      setDeleteError(null);
      setDeleting(false);
    }
  };

  return (
    <div className="space-y-10">
      <div>
        <h1 className="text-2xl font-semibold text-gray-900">{t('securityTitle')}</h1>
        <p className="mt-2 text-gray-500">{t('securityDescription')}</p>
      </div>

      <section className="space-y-4">
        <h2 className="text-lg font-medium text-gray-900">{t('changeEmail')}</h2>
        {emailError && (
          <Alert variant="destructive">
            <AlertDescription>{emailError}</AlertDescription>
          </Alert>
        )}
        {emailSuccess && (
          <Alert variant="default">
            <AlertDescription>{t('emailUpdated')}</AlertDescription>
          </Alert>
        )}
        <form onSubmit={saveEmail} className="max-w-2xl space-y-4">
          {currentEmail && (
            <div className="flex flex-col gap-1.5">
              <Label>{t('currentEmailLabel')}</Label>
              <p className="text-sm text-gray-600">{currentEmail}</p>
            </div>
          )}
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="new-email">{t('newEmailAddress')}</Label>
            <Input
              id="new-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="new@example.com"
            />
          </div>
          <Button type="submit" disabled={savingEmail}>
            {savingEmail ? tCommon('saving') : t('updateEmail')}
          </Button>
        </form>
      </section>

      <section className="space-y-4">
        <h2 className="text-lg font-medium text-gray-900">{t('changePassword')}</h2>
        {passwordError && (
          <Alert variant="destructive">
            <AlertDescription>{passwordError}</AlertDescription>
          </Alert>
        )}
        {passwordSuccess && (
          <Alert variant="default">
            <AlertDescription>{t('passwordChanged')}</AlertDescription>
          </Alert>
        )}
        <form onSubmit={savePassword} className="max-w-2xl space-y-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="current-password">{t('currentPassword')}</Label>
            <Input
              id="current-password"
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="new-password">{t('newPassword')}</Label>
            <Input
              id="new-password"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
            />
          </div>
          <Button type="submit" disabled={savingPassword}>
            {savingPassword ? tCommon('saving') : t('changePassword')}
          </Button>
        </form>
      </section>

      <section className="space-y-4">
        <h2 className="text-lg font-medium text-gray-900">{tApps('title')}</h2>
        <p className="text-sm text-gray-500">{tApps('description')}</p>
        {appsError && (
          <Alert variant="destructive">
            <AlertDescription>{appsError}</AlertDescription>
          </Alert>
        )}
        {apps !== null && (
          <Card>
            <CardContent>
              {apps.length === 0 ? (
                <EmptyState message={tApps('emptyState')} />
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{tApps('columnName')}</TableHead>
                      <TableHead>{tApps('columnAuthorizedAt')}</TableHead>
                      <TableHead className="text-right">{tApps('columnActions')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {apps.map((app) => (
                      <TableRow key={app.id}>
                        <TableCell className="font-medium">
                          {app.website ? (
                            <Button variant="link" size="sm" className="h-auto p-0 font-medium" asChild>
                              <a href={app.website} target="_blank" rel="noreferrer noopener">
                                {app.name}
                              </a>
                            </Button>
                          ) : (
                            app.name
                          )}
                          {isCurrentSession(app) && (
                            <Badge variant="secondary" className="ml-2">
                              {tApps('currentSessionBadge')}
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="text-muted-foreground">{formatDate(app.created_at)}</TableCell>
                        <TableCell className="text-right">
                          <Button
                            type="button"
                            variant="link"
                            size="sm"
                            onClick={() => openRevoke(app)}
                            className="text-destructive"
                          >
                            {tApps('revokeButton')}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        )}
        <Dialog open={revokeTarget !== null} onOpenChange={(open) => !open && openRevoke(null)}>
          <DialogContent>
            <form onSubmit={submitRevoke} className="space-y-4">
              <DialogHeader>
                <DialogTitle>{tApps('revokeConfirmTitle', { name: revokeTarget?.name ?? '' })}</DialogTitle>
                <DialogDescription>
                  {tApps('revokeConfirmBody', { name: revokeTarget?.name ?? '' })}
                </DialogDescription>
              </DialogHeader>
              {revokeTarget && isCurrentSession(revokeTarget) && (
                <Alert variant="default">
                  <AlertDescription>{tApps('revokeCurrentWarning')}</AlertDescription>
                </Alert>
              )}
              {revokeError && (
                <Alert variant="destructive">
                  <AlertDescription>{revokeError}</AlertDescription>
                </Alert>
              )}
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => openRevoke(null)} disabled={revoking}>
                  {tCommon('cancel')}
                </Button>
                <Button type="submit" variant="destructive" disabled={revoking}>
                  {revoking ? tCommon('saving') : tApps('revokeConfirmButton')}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </section>

      <section className="space-y-4 border-t pt-10">
        <h2 className="text-lg font-medium text-red-700">{t('dangerZoneTitle')}</h2>
        <div className="max-w-2xl space-y-3 rounded-lg border border-red-200 bg-red-50 p-4">
          <h3 className="font-medium text-gray-900">{t('deleteAccountHeading')}</h3>
          <p className="text-sm text-gray-700">{t('deleteAccountBlurb')}</p>
          <Dialog open={deleteOpen} onOpenChange={openDelete}>
            <DialogTrigger asChild>
              <Button variant="destructive">{t('deleteAccountButton')}</Button>
            </DialogTrigger>
            <DialogContent>
              <form onSubmit={submitDelete} className="space-y-4">
                <DialogHeader>
                  <DialogTitle>{t('deleteAccountDialogTitle')}</DialogTitle>
                  <DialogDescription>{t('deleteAccountDialogBody')}</DialogDescription>
                </DialogHeader>
                {deleteError && (
                  <Alert variant="destructive">
                    <AlertDescription>{deleteError}</AlertDescription>
                  </Alert>
                )}
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="delete-password">{t('currentPassword')}</Label>
                  <Input
                    id="delete-password"
                    type="password"
                    autoComplete="current-password"
                    value={deletePassword}
                    onChange={(e) => setDeletePassword(e.target.value)}
                    required
                  />
                </div>
                <DialogFooter>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => openDelete(false)}
                    disabled={deleting}
                  >
                    {tCommon('cancel')}
                  </Button>
                  <Button type="submit" variant="destructive" disabled={deleting || !deletePassword}>
                    {deleting ? tCommon('saving') : t('deleteAccountConfirmButton')}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </div>
      </section>
    </div>
  );
}
