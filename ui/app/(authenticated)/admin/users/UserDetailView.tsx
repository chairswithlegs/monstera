'use client';

import {
  getUser,
  setUserRole,
  deleteUser,
  type AdminUser,
} from '@/lib/api/admin';
import { getUser as getCurrentUser } from '@/lib/api/user';
import type { User } from '@/lib/api/user';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';

const roleOptions = ['user', 'moderator', 'admin'] as const;

type Props = { id: string };

export default function AdminUserDetailView({ id }: Props) {
  const router = useRouter();
  const [user, setUser] = useState<AdminUser | null>(null);
  const [currentActor, setCurrentActor] = useState<User | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);
  const [roleDialogOpen, setRoleDialogOpen] = useState(false);
  const [newRole, setNewRole] = useState('user');

  const load = useCallback(() => {
    if (!id) return;
    getUser(id)
      .then(setUser)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, [id]);

  useEffect(() => {
    load();
    getCurrentUser()
      .then(setCurrentActor)
      .catch(() => setCurrentActor(null));
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

  const handleSetRole = async () => {
    if (!user) return;
    await act(() => setUserRole(user.account_id, newRole));
    setRoleDialogOpen(false);
  };

  const handleDelete = async () => {
    if (!user) return;
    setActing(true);
    try {
      await deleteUser(user.account_id);
      router.push('/admin/users');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Action failed');
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

  const isSelf = currentActor !== null && user.account_id === currentActor.account_id;

  return (
    <div>
      <p className="mb-4">
        <Button variant="link" size="sm" className="h-auto p-0" asChild>
          <Link href="/admin/users">← Back to users</Link>
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
      {isSelf && (
        <p className="mt-3 text-sm text-muted-foreground">Role and delete actions are disabled for your own account.</p>
      )}
      <div className="mt-6 flex flex-wrap gap-2">
        <Dialog
          open={roleDialogOpen}
          onOpenChange={(open) => {
            setRoleDialogOpen(open);
            if (open) setNewRole(user.role);
          }}
        >
          <DialogTrigger asChild>
            <Button type="button" variant="secondary" disabled={acting || isSelf}>
              Set role
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Set role</DialogTitle>
              <DialogDescription>Choose a new role for @{user.username}.</DialogDescription>
            </DialogHeader>
            <div className="grid gap-2 py-4">
              <Label htmlFor="new-role">New role</Label>
              <select
                id="new-role"
                value={newRole}
                onChange={(e) => setNewRole(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                {roleOptions.map((r) => (
                  <option key={r} value={r}>{r}</option>
                ))}
              </select>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setRoleDialogOpen(false)}>
                Cancel
              </Button>
              <Button onClick={handleSetRole} disabled={acting}>
                {acting ? 'Saving…' : 'Save'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button type="button" variant="destructive" disabled={acting || isSelf} className="bg-red-800 hover:bg-red-900">
              Delete account
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete account?</AlertDialogTitle>
              <AlertDialogDescription>
                This action cannot be undone. This will permanently delete the account and all associated data.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                Delete
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </div>
  );
}
