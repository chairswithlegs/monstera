'use client';

import {
  getUser,
  suspendUser,
  unsuspendUser,
  silenceUser,
  unsilenceUser,
  setUserRole,
  deleteUser,
  type AdminUser,
} from '@/lib/api/admin';
import { useModeratorUser } from '@/contexts/moderator-user';
import { isAdmin } from '@/lib/api/user';
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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

type Props = { id: string };

export default function UserDetailView({ id }: Props) {
  const router = useRouter();
  const currentUser = useModeratorUser();
  const [user, setUser] = useState<AdminUser | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);
  const [roleDialogOpen, setRoleDialogOpen] = useState(false);
  const [newRole, setNewRole] = useState('');

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

  const handleSetRole = async () => {
    if (!user || !newRole.trim()) return;
    await act(() => setUserRole(user.account_id, newRole.trim()));
    setRoleDialogOpen(false);
    setNewRole('');
  };

  const handleDelete = async () => {
    if (!user) return;
    await act(() => deleteUser(user.account_id));
    router.push('/moderator/users');
  };

  if (error && !user) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!user) return <p className="text-muted-foreground">Loading…</p>;

  const showAdminActions = currentUser && isAdmin(currentUser);

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
          <Button type="button" disabled={acting} onClick={() => act(() => unsuspendUser(user.account_id))}>
            Unsuspend
          </Button>
        ) : (
          <Button type="button" variant="destructive" disabled={acting} onClick={() => act(() => suspendUser(user.account_id))}>
            Suspend
          </Button>
        )}
        {user.silenced ? (
          <Button type="button" disabled={acting} onClick={() => act(() => unsilenceUser(user.account_id))}>
            Unsilence
          </Button>
        ) : (
          <Button type="button" variant="secondary" disabled={acting} onClick={() => act(() => silenceUser(user.account_id))} className="bg-amber-600 text-white hover:bg-amber-700 hover:text-white">
            Silence
          </Button>
        )}
        {showAdminActions && (
          <>
            <Dialog open={roleDialogOpen} onOpenChange={setRoleDialogOpen}>
              <DialogTrigger asChild>
                <Button type="button" variant="secondary" disabled={acting}>
                  Set role
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Set role</DialogTitle>
                  <DialogDescription>Enter the new role (user, moderator, admin).</DialogDescription>
                </DialogHeader>
                <div className="grid gap-2 py-4">
                  <Label htmlFor="new-role">New role</Label>
                  <Input
                    id="new-role"
                    value={newRole}
                    onChange={(e) => setNewRole(e.target.value)}
                    placeholder={user.role}
                  />
                </div>
                <DialogFooter>
                  <Button variant="outline" onClick={() => setRoleDialogOpen(false)}>
                    Cancel
                  </Button>
                  <Button onClick={handleSetRole} disabled={acting || !newRole.trim()}>
                    {acting ? 'Saving…' : 'Save'}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button type="button" variant="destructive" disabled={acting} className="bg-red-800 hover:bg-red-900">
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
          </>
        )}
      </div>
    </div>
  );
}
