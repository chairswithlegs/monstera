'use client';

import { patchEmail, patchPassword } from '@/lib/api/user';
import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { translateApiError } from '@/lib/i18n/errors';

export default function SecurityPage() {
  const t = useTranslations('account');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const [email, setEmail] = useState('');
  const [emailError, setEmailError] = useState<string | null>(null);
  const [emailSuccess, setEmailSuccess] = useState(false);
  const [savingEmail, setSavingEmail] = useState(false);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [passwordSuccess, setPasswordSuccess] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);

  const saveEmail = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingEmail(true);
    setEmailError(null);
    setEmailSuccess(false);
    try {
      await patchEmail({ email });
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
    </div>
  );
}
