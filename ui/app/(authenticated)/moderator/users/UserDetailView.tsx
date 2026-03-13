'use client';

import {
  getUser,
  suspendUser,
  unsuspendUser,
  silenceUser,
  unsilenceUser,
  type AdminUser,
} from '@/lib/api/admin';
import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';

type Props = { id: string };

export default function UserDetailView({ id }: Props) {
  const [user, setUser] = useState<AdminUser | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);

  const load = useCallback(() => {
    if (!id) return;
    getUser(id)
      .then(setUser)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const act = async (fn: () => Promise<void>) => {
    setActing(true);
    try {
      await fn();
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Action failed');
    } finally {
      setActing(false);
    }
  };

  if (error && !user) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!user) return <p className="text-muted-foreground">Loading…</p>;

  const isAdmin = user.role === 'admin';

  return (
    <div>
      <p className="mb-4">
        <Button variant="link" size="sm" className="h-auto p-0" asChild>
          <Link href="/moderator/users">← Back to users</Link>
        </Button>
      </p>
      <h1 className="text-2xl font-semibold text-foreground">@{user.username}</h1>
      <p className="mt-1 text-muted-foreground">{user.email}</p>
      <p className="mt-1 text-sm text-muted-foreground">Account ID: {user.account_id}</p>
      <p className="mt-1 text-sm text-muted-foreground">Role: {user.role}</p>
      {user.suspended && <p className="mt-2 text-destructive">Suspended</p>}
      {user.silenced && !user.suspended && <p className="mt-2 text-amber-600 dark:text-amber-500">Silenced</p>}
      {error && (
        <Alert variant="destructive" className="mt-2">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <div className="mt-6 flex flex-wrap gap-2">
        {user.suspended ? (
          <Button type="button" disabled={acting || isAdmin} onClick={() => act(() => unsuspendUser(user.account_id))}>
            Unsuspend
          </Button>
        ) : (
          <Button type="button" variant="destructive" disabled={acting || isAdmin} onClick={() => act(() => suspendUser(user.account_id))}>
            Suspend
          </Button>
        )}
        {user.silenced ? (
          <Button type="button" disabled={acting || isAdmin} onClick={() => act(() => unsilenceUser(user.account_id))}>
            Unsilence
          </Button>
        ) : (
          <Button type="button" variant="secondary" disabled={acting || isAdmin} onClick={() => act(() => silenceUser(user.account_id))} className="bg-amber-600 text-white hover:bg-amber-700 hover:text-white">
            Silence
          </Button>
        )}
      </div>
      {isAdmin && (
        <p className="mt-4 text-sm text-muted-foreground">Suspend and silence actions are not available for admin accounts.</p>
      )}
    </div>
  );
}
