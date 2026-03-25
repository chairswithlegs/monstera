'use client';

import { getUser, patchProfile } from '@/lib/api/user';
import type { ProfileField } from '@/lib/api/user';
import { useCallback, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';

export default function ProfilePage() {
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
        setDisplayName(u.display_name ?? '');
        setNote(u.note ?? '');
        setLocked(u.locked);
        setBot(u.bot);
        const rawFields: ProfileField[] = u.fields ?? [];
        setFields(rawFields.map((f) => ({ name: f.name, value: f.value })));
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false));
  }, []);

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
      setError(e instanceof Error ? e.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="text-muted-foreground">Loading...</div>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Profile</h1>
      <p className="mt-2 text-gray-500">Update your public profile information.</p>
      {error && (
        <Alert variant="destructive" className="mt-4">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {success && (
        <Alert variant="default" className="mt-4">
          <AlertDescription>Profile saved.</AlertDescription>
        </Alert>
      )}
      <form onSubmit={save} className="mt-6 max-w-2xl space-y-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="display-name">Display name</Label>
          <p className="text-xs text-muted-foreground">Your name as it appears on your profile and in posts. Leave blank to use your username.</p>
          <Input
            id="display-name"
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Your name"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="bio">Bio</Label>
          <p className="text-xs text-muted-foreground">A short description of yourself shown on your public profile.</p>
          <textarea
            id="bio"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={4}
            className="flex min-h-[60px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
            placeholder="Tell people about yourself."
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
            <Label htmlFor="locked">Require follow requests</Label>
          </div>
          <p className="text-xs text-muted-foreground">When enabled, new followers must be approved before they can see your posts.</p>
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
            <Label htmlFor="bot">This is a bot account</Label>
          </div>
          <p className="text-xs text-muted-foreground">Marks your account as automated. Displayed as a badge on your profile across the fediverse.</p>
        </div>
        <div className="flex flex-col gap-2">
          <div className="flex flex-col gap-1">
            <Label>Profile fields</Label>
            <p className="text-xs text-muted-foreground">Up to 4 custom label–value pairs shown on your profile, e.g. &ldquo;Website&rdquo; or &ldquo;Pronouns&rdquo;.</p>
          </div>
          {fields.map((f, i) => (
            <div key={i} className="flex gap-2">
              <Input
                placeholder="Label"
                value={f.name}
                onChange={(e) => updateField(i, 'name', e.target.value)}
                className="w-40"
              />
              <Input
                placeholder="Content"
                value={f.value}
                onChange={(e) => updateField(i, 'value', e.target.value)}
                className="flex-1"
              />
              <Button type="button" variant="outline" onClick={() => removeField(i)}>
                Remove
              </Button>
            </div>
          ))}
          {fields.length < 4 && (
            <Button type="button" variant="outline" onClick={addField} className="self-start">
              Add field
            </Button>
          )}
        </div>
        <Button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </form>
    </div>
  );
}
