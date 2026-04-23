'use client';

import { deleteAccount, getUser, patchEmail, patchPassword } from '@/lib/api/user';
import { useCallback, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';
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
