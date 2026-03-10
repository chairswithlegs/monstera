'use client';

import { patchEmail, patchPassword } from '@/lib/api/user';
import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';

export default function SecurityPage() {
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
      setEmailError(e instanceof Error ? e.message : 'Failed to update email');
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
      setPasswordError(e instanceof Error ? e.message : 'Failed to change password');
    } finally {
      setSavingPassword(false);
    }
  };

  return (
    <div className="space-y-10">
      <div>
        <h1 className="text-2xl font-semibold text-gray-900">Security</h1>
        <p className="mt-2 text-gray-500">Manage your login credentials.</p>
      </div>

      <section className="space-y-4">
        <h2 className="text-lg font-medium text-gray-900">Change email</h2>
        {emailError && (
          <Alert variant="destructive">
            <AlertDescription>{emailError}</AlertDescription>
          </Alert>
        )}
        {emailSuccess && (
          <Alert variant="default">
            <AlertDescription>Email updated.</AlertDescription>
          </Alert>
        )}
        <form onSubmit={saveEmail} className="max-w-2xl space-y-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="new-email">New email address</Label>
            <Input
              id="new-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="new@example.com"
            />
          </div>
          <Button type="submit" disabled={savingEmail}>
            {savingEmail ? 'Saving…' : 'Update email'}
          </Button>
        </form>
      </section>

      <section className="space-y-4">
        <h2 className="text-lg font-medium text-gray-900">Change password</h2>
        {passwordError && (
          <Alert variant="destructive">
            <AlertDescription>{passwordError}</AlertDescription>
          </Alert>
        )}
        {passwordSuccess && (
          <Alert variant="default">
            <AlertDescription>Password changed.</AlertDescription>
          </Alert>
        )}
        <form onSubmit={savePassword} className="max-w-2xl space-y-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="current-password">Current password</Label>
            <Input
              id="current-password"
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="new-password">New password</Label>
            <Input
              id="new-password"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
            />
          </div>
          <Button type="submit" disabled={savingPassword}>
            {savingPassword ? 'Saving…' : 'Change password'}
          </Button>
        </form>
      </section>
    </div>
  );
}
