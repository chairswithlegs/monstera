'use client';

import { getUser, patchPreferences } from '@/lib/api/user';
import { useCallback, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';

const visibilityOptions = [
  { value: 'public', label: 'Public' },
  { value: 'unlisted', label: 'Unlisted' },
  { value: 'private', label: 'Followers only' },
  { value: 'direct', label: 'Direct message' },
];

const quotePolicyOptions = [
  { value: 'public', label: 'Anyone' },
  { value: 'followers', label: 'Followers only' },
  { value: 'nobody', label: 'Nobody' },
];

export default function PreferencesPage() {
  const [defaultPrivacy, setDefaultPrivacy] = useState('public');
  const [defaultSensitive, setDefaultSensitive] = useState(false);
  const [defaultLanguage, setDefaultLanguage] = useState('');
  const [defaultQuotePolicy, setDefaultQuotePolicy] = useState('public');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const load = useCallback(() => {
    getUser()
      .then((u) => {
        setDefaultPrivacy(u.default_privacy || 'public');
        setDefaultSensitive(u.default_sensitive);
        setDefaultLanguage(u.default_language || '');
        setDefaultQuotePolicy(u.default_quote_policy || 'public');
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      await patchPreferences({
        default_privacy: defaultPrivacy,
        default_sensitive: defaultSensitive,
        default_language: defaultLanguage,
        default_quote_policy: defaultQuotePolicy,
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
      <h1 className="text-2xl font-semibold text-gray-900">Preferences</h1>
      <p className="mt-2 text-gray-500">Configure defaults for new posts.</p>
      {error && (
        <Alert variant="destructive" className="mt-4">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {success && (
        <Alert variant="default" className="mt-4">
          <AlertDescription>Preferences saved.</AlertDescription>
        </Alert>
      )}
      <form onSubmit={save} className="mt-6 max-w-2xl space-y-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="default-privacy">Default post visibility</Label>
          <p className="text-xs text-muted-foreground">Controls who can see new posts. Public appears on timelines; Unlisted is public but omitted from timelines; Followers only is private to your followers; Direct is only visible to mentioned accounts.</p>
          <select
            id="default-privacy"
            value={defaultPrivacy}
            onChange={(e) => setDefaultPrivacy(e.target.value)}
            className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            {visibilityOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="default-language">Default post language</Label>
          <p className="text-xs text-muted-foreground">BCP-47 language tag for new posts (e.g. <code className="font-mono">en</code>, <code className="font-mono">fr</code>, <code className="font-mono">zh-Hant</code>). Used by clients and screen readers to present content in the correct language.</p>
          <Input
            id="default-language"
            type="text"
            value={defaultLanguage}
            onChange={(e) => setDefaultLanguage(e.target.value)}
            placeholder="en"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center gap-2">
            <input
              id="default-sensitive"
              type="checkbox"
              checked={defaultSensitive}
              onChange={(e) => setDefaultSensitive(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300"
            />
            <Label htmlFor="default-sensitive">Mark media as sensitive by default</Label>
          </div>
          <p className="text-xs text-muted-foreground">When enabled, images and videos in new posts will be hidden behind a content warning until the viewer clicks to reveal them.</p>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="default-quote-policy">Who can quote your posts</Label>
          <p className="text-xs text-muted-foreground">Controls who is allowed to quote your posts on compatible servers. This applies as the default for new posts and can be overridden per post.</p>
          <select
            id="default-quote-policy"
            value={defaultQuotePolicy}
            onChange={(e) => setDefaultQuotePolicy(e.target.value)}
            className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            {quotePolicyOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>
        <Button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </form>
    </div>
  );
}
