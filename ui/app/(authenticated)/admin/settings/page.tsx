'use client';

import { getSettings, putSettings } from '@/lib/api/admin';
import { getUser, isAdminOrModerator } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';

export default function ServerSettingsPage() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
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
      .then((r) => setSettings(r.settings ?? {}))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, []);

  useEffect(() => {
    if (user && isAdminOrModerator(user)) load();
  }, [user, load]);

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await putSettings(settings);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  if (loading || !user || !isAdminOrModerator(user)) {
    return null;
  }

  const keys = Object.keys(settings).sort();
  const editable: Record<string, string> = { ...settings };

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Server settings</h1>
      <p className="mt-2 text-gray-500">Instance configuration (admin only).</p>
      {error && (
        <Alert variant="destructive" className="mt-2">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <form onSubmit={save} className="mt-6 max-w-2xl space-y-4">
        {keys.length === 0 ? (
          <p className="text-muted-foreground">No settings yet.</p>
        ) : (
          keys.map((key) => (
            <div key={key} className="flex flex-col gap-1.5">
              <Label htmlFor={`setting-${key}`}>{key}</Label>
              <Input
                id={`setting-${key}`}
                type="text"
                value={editable[key] ?? ''}
                onChange={(e) => setSettings((s) => ({ ...s, [key]: e.target.value }))}
              />
            </div>
          ))
        )}
        <Button type="submit" disabled={saving || keys.length === 0}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </form>
    </div>
  );
}
