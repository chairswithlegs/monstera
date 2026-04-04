'use client';

import { getSettings, putSettings } from '@/lib/api/admin';
import { getUser, isAdminOrModerator } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { translateApiError } from '@/lib/i18n/errors';

export default function ServerSettingsPage() {
  const router = useRouter();
  const t = useTranslations('admin');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [registrationMode, setRegistrationMode] = useState('closed');
  const [inviteMaxUses, setInviteMaxUses] = useState('');
  const [inviteExpiresInDays, setInviteExpiresInDays] = useState('');
  const [serverName, setServerName] = useState('');
  const [serverDescription, setServerDescription] = useState('');
  const [serverRules, setServerRules] = useState('');
  const [trendingLinksScope, setTrendingLinksScope] = useState('disabled');
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [saving, setSaving] = useState(false);

  const registrationModeOptions = [
    { value: 'closed', label: t('modeClosed') },
    { value: 'open', label: t('modeOpen') },
    { value: 'approval', label: t('modeApproval') },
    { value: 'invite', label: t('modeInvite') },
  ];

  const trendingLinksScopeOptions = [
    { value: 'disabled', label: t('trendingLinksScopeDisabled') },
    { value: 'users', label: t('trendingLinksScopeUsers') },
    { value: 'all', label: t('trendingLinksScopeAll') },
  ];

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
        setServerName(s.server_name ?? '');
        setServerDescription(s.server_description ?? '');
        setServerRules(s.server_rules?.join('\n') ?? '');
        setTrendingLinksScope(s.trending_links_scope ?? 'disabled');
      })
      .catch((e) => setError(translateApiError(tErr, e)));
  }, [tErr]);

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
        server_name: serverName.trim() || null,
        server_description: serverDescription.trim() || null,
        server_rules: serverRules.split('\n').map((r) => r.trim()).filter(Boolean),
        trending_links_scope: trendingLinksScope,
      });
      setSuccess(true);
    } catch (e) {
      setError(translateApiError(tErr, e));
    } finally {
      setSaving(false);
    }
  };

  if (loading || !user || !isAdminOrModerator(user)) {
    return null;
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">{t('settingsTitle')}</h1>
      <p className="mt-2 text-gray-500">{t('settingsDescription')}</p>
      {error && (
        <Alert variant="destructive" className="mt-4">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {success && (
        <Alert variant="default" className="mt-4">
          <AlertDescription>{t('settingsSaved')}</AlertDescription>
        </Alert>
      )}
      <form onSubmit={save} className="mt-6 max-w-2xl space-y-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="registration-mode">{t('registrationMode')}</Label>
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
              <Label htmlFor="invite-max-uses">{t('inviteMaxUses')}</Label>
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
              <Label htmlFor="invite-expires-in-days">{t('inviteExpiry')}</Label>
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
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="server-name">
            {t('serverName')}{' '}
            <span className="text-muted-foreground text-xs">{serverName.length}/24</span>
          </Label>
          <Input
            id="server-name"
            type="text"
            maxLength={24}
            placeholder="My Monstera"
            value={serverName}
            onChange={(e) => setServerName(e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="server-description">{t('serverDescription')}</Label>
          <textarea
            id="server-description"
            value={serverDescription}
            onChange={(e) => setServerDescription(e.target.value)}
            rows={3}
            className="flex min-h-[60px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
            placeholder="A short description of your server."
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="server-rules">{t('serverRules')}</Label>
          <textarea
            id="server-rules"
            value={serverRules}
            onChange={(e) => setServerRules(e.target.value)}
            rows={5}
            className="flex min-h-[60px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
            placeholder="Be respectful.&#10;No spam."
          />
          <p className="text-xs text-muted-foreground">{t('serverRulesHint')}</p>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="trending-links-scope">{t('trendingLinksScope')}</Label>
          <select
            id="trending-links-scope"
            value={trendingLinksScope}
            onChange={(e) => setTrendingLinksScope(e.target.value)}
            className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            {trendingLinksScopeOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>
        <Button type="submit" disabled={saving}>
          {saving ? tCommon('saving') : tCommon('save')}
        </Button>
      </form>
    </div>
  );
}
