'use client';

import { getSettings, putSettings } from '@/lib/api/admin';
import { getUser, isAdminOrModerator } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Alert, AlertDescription } from '@/components/ui/alert';

const registrationModeOptions = [
  { value: 'closed', label: 'Closed (no new registrations)' },
  { value: 'open', label: 'Open (anyone can register)' },
  { value: 'approval', label: 'Approval required' },
  { value: 'invite', label: 'Invite only' },
] as const;

export default function ServerSettingsPage() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [registrationMode, setRegistrationMode] = useState('closed');
  const [inviteMaxUses, setInviteMaxUses] = useState('');
  const [inviteExpiresInDays, setInviteExpiresInDays] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    getUser()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!user || !isAdminOrModerator(user)) {
      router.replace('/home');
    }
  }, [loading, user, router]);

  const load = useCallback(() => {
    getSettings()
      .then((s) => {
        setRegistrationMode(s.registration_mode ?? 'closed');
        setInviteMaxUses(s.invite_max_uses != null ? String(s.invite_max_uses) : '');
        setInviteExpiresInDays(s.invite_expires_in_days != null ? String(s.invite_expires_in_days) : '');
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, []);

  useEffect(() => {
    if (user && isAdminOrModerator(user)) load();
  }, [user, load]);

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      await putSettings({
        registration_mode: registrationMode,
        invite_max_uses: inviteMaxUses !== '' ? Number(inviteMaxUses) : null,
        invite_expires_in_days: inviteExpiresInDays !== '' ? Number(inviteExpiresInDays) : null,
      });
      setSuccess(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  if (loading || !user || !isAdminOrModerator(user)) {
    return null;
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Server settings</h1>
      <p className="mt-2 text-gray-500">Instance configuration (admin only).</p>
      {error && (
        <Alert variant="destructive" className="mt-4">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {success && (
        <Alert variant="default" className="mt-4">
          <AlertDescription>Settings saved.</AlertDescription>
        </Alert>
      )}
      <form onSubmit={save} className="mt-6 max-w-2xl space-y-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="registration-mode">Registration mode</Label>
          <select
            id="registration-mode"
            value={registrationMode}
            onChange={(e) => setRegistrationMode(e.target.value)}
            className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            {registrationModeOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>
        {registrationMode === 'invite' && (
          <>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="invite-max-uses">Default max uses per invite</Label>
              <Input
                id="invite-max-uses"
                type="number"
                min={1}
                placeholder="Unlimited"
                value={inviteMaxUses}
                onChange={(e) => setInviteMaxUses(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="invite-expires-in-days">Default invite expiry (days)</Label>
              <Input
                id="invite-expires-in-days"
                type="number"
                min={1}
                placeholder="Never"
                value={inviteExpiresInDays}
                onChange={(e) => setInviteExpiresInDays(e.target.value)}
              />
            </div>
          </>
        )}
        <Button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </form>
    </div>
  );
}
