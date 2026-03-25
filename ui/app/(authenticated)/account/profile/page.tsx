'use client';

import { getUser, patchProfile } from '@/lib/api/user';
import type { ProfileField } from '@/lib/api/user';
import { useCallback, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { translateApiError } from '@/lib/i18n/errors';

export default function ProfilePage() {
  const t = useTranslations('account');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const [username, setUsername] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [note, setNote] = useState('');
  const [locked, setLocked] = useState(false);
  const [bot, setBot] = useState(false);
  const [fields, setFields] = useState<{ name: string; value: string }[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const load = useCallback(() => {
    getUser()
      .then((u) => {
        setUsername(u.username);
        setDisplayName(u.display_name ?? '');
        setNote(u.note ?? '');
        setLocked(u.locked);
        setBot(u.bot);
        const rawFields: ProfileField[] = u.fields ?? [];
        setFields(rawFields.map((f) => ({ name: f.name, value: f.value })));
      })
      .catch((e) => setError(translateApiError(tErr, e)))
      .finally(() => setLoading(false));
  }, [tErr]);

  useEffect(() => {
    load();
  }, [load]);

  const addField = () => {
    if (fields.length < 4) {
      setFields([...fields, { name: '', value: '' }]);
    }
  };

  const removeField = (i: number) => {
    setFields(fields.filter((_, idx) => idx !== i));
  };

  const updateField = (i: number, key: 'name' | 'value', val: string) => {
    setFields(fields.map((f, idx) => (idx === i ? { ...f, [key]: val } : f)));
  };

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      await patchProfile({
        display_name: displayName.trim() || null,
        note: note.trim() || null,
        locked,
        bot,
        fields: fields
          .filter((f) => f.name.trim())
          .map((f) => ({ name: f.name.trim(), value: f.value.trim(), verified_at: null })),
      });
      setSuccess(true);
    } catch (e) {
      setError(translateApiError(tErr, e));
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="text-muted-foreground">{tCommon('loading')}</div>;

  return (
    <div>
      <div className="flex items-baseline justify-between gap-4">
        <h1 className="text-2xl font-semibold text-gray-900">{t('profileTitle')}</h1>
        {username && (
          <Button variant="link" size="sm" className="h-auto p-0" asChild>
            <Link href={`/public/profile?u=${encodeURIComponent(username)}`} target="_blank">
              {t('viewPublicProfile')}
            </Link>
          </Button>
        )}
      </div>
      <p className="mt-2 text-gray-500">{t('profileDescription')}</p>
      {error && (
        <Alert variant="destructive" className="mt-4">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {success && (
        <Alert variant="default" className="mt-4">
          <AlertDescription>{t('profileSaved')}</AlertDescription>
        </Alert>
      )}
      <form onSubmit={save} className="mt-6 max-w-2xl space-y-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="display-name">{t('displayName')}</Label>
          <p className="text-xs text-muted-foreground">{t('displayNameHint')}</p>
          <Input
            id="display-name"
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder={t('displayNamePlaceholder')}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="bio">{t('bio')}</Label>
          <p className="text-xs text-muted-foreground">{t('bioHint')}</p>
          <textarea
            id="bio"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={4}
            className="flex min-h-[60px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
            placeholder={t('bioPlaceholder')}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center gap-2">
            <input
              id="locked"
              type="checkbox"
              checked={locked}
              onChange={(e) => setLocked(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300"
            />
            <Label htmlFor="locked">{t('requireFollowRequests')}</Label>
          </div>
          <p className="text-xs text-muted-foreground">{t('requireFollowRequestsHint')}</p>
        </div>
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center gap-2">
            <input
              id="bot"
              type="checkbox"
              checked={bot}
              onChange={(e) => setBot(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300"
            />
            <Label htmlFor="bot">{t('botAccount')}</Label>
          </div>
          <p className="text-xs text-muted-foreground">{t('botAccountHint')}</p>
        </div>
        <div className="flex flex-col gap-2">
          <div className="flex flex-col gap-1">
            <Label>{t('profileFields')}</Label>
            <p className="text-xs text-muted-foreground">{t('profileFieldsHint')}</p>
          </div>
          {fields.map((f, i) => (
            <div key={i} className="flex gap-2">
              <Input
                placeholder={t('labelPlaceholder')}
                value={f.name}
                onChange={(e) => updateField(i, 'name', e.target.value)}
                className="w-40"
              />
              <Input
                placeholder={t('contentPlaceholder')}
                value={f.value}
                onChange={(e) => updateField(i, 'value', e.target.value)}
                className="flex-1"
              />
              <Button type="button" variant="outline" onClick={() => removeField(i)}>
                {tCommon('remove')}
              </Button>
            </div>
          ))}
          {fields.length < 4 && (
            <Button type="button" variant="outline" onClick={addField} className="self-start">
              {t('addField')}
            </Button>
          )}
        </div>
        <Button type="submit" disabled={saving}>
          {saving ? tCommon('saving') : tCommon('save')}
        </Button>
      </form>
    </div>
  );
}
