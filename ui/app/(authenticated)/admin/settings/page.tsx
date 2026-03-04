'use client';

import { getSettings, putSettings } from '@/lib/api/admin';
import { getUser, isAdminOrModerator } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useState } from 'react';

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
      {error && <p className="mt-2 text-red-600">{error}</p>}
      <form onSubmit={save} className="mt-6 max-w-2xl space-y-4">
        {keys.length === 0 ? (
          <p className="text-gray-500">No settings yet.</p>
        ) : (
          keys.map((key) => (
            <div key={key}>
              <label className="block text-sm font-medium text-gray-700">{key}</label>
              <input
                type="text"
                value={editable[key] ?? ''}
                onChange={(e) => setSettings((s) => ({ ...s, [key]: e.target.value }))}
                className="mt-1 w-full rounded border border-gray-300 px-3 py-1.5 text-sm"
              />
            </div>
          ))
        )}
        <button
          type="submit"
          disabled={saving || keys.length === 0}
          className="rounded bg-indigo-600 px-3 py-1.5 text-sm text-white hover:bg-indigo-500 disabled:opacity-50"
        >
          {saving ? 'Saving…' : 'Save'}
        </button>
      </form>
    </div>
  );
}
