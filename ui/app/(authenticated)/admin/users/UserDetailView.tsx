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
import { useTranslations } from 'next-intl';
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
import { translateApiError } from '@/lib/i18n/errors';

const roleOptions = ['user', 'moderator', 'admin'] as const;

type Props = { id: string };

export default function AdminUserDetailView({ id }: Props) {
  const router = useRouter();
  const t = useTranslations('admin');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const [user, setUser] = useState<AdminUser | null>(null);
  const [currentActor, setCurrentActor] = useState<User | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);
  const [roleDialogOpen, setRoleDialogOpen] = useState(false);
  const [newRole, setNewRole] = useState('user');
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleteConfirmInput, setDeleteConfirmInput] = useState('');

  const load = useCallback(() => {
    if (!id) return;
    getUser(id)
      .then(setUser)
      .catch((e) => setError(translateApiError(tErr, e)));
  }, [id, tErr]);

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
      setError(translateApiError(tErr, e));
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
      setError(translateApiError(tErr, e));
      setActing(false);
      setDeleteDialogOpen(false);
    }
  };

  if (error && !user) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!user) return <p className="text-muted-foreground">{tCommon('loading')}</p>;

  const isSelf = currentActor !== null && user.account_id === currentActor.account_id;

  return (
    <div>
      <p className="mb-4">
        <Button variant="link" size="sm" className="h-auto p-0" asChild>
          <Link href="/admin/users">{t('backToUsers')}</Link>
        </Button>
      </p>
      <h1 className="text-2xl font-semibold text-foreground">@{user.username}</h1>
      <p className="mt-1 text-muted-foreground">{user.email}</p>
      <p className="mt-1 text-sm text-muted-foreground">{t('accountId', { id: user.account_id })}</p>
      <p className="mt-1 text-sm text-muted-foreground">{t('role', { role: user.role })}</p>
      {user.suspended && <p className="mt-2 text-destructive">{t('badgeSuspended')}</p>}
      {user.silenced && !user.suspended && <p className="mt-2 text-amber-600 dark:text-amber-500">{t('badgeSilenced')}</p>}
      {error && (
        <Alert variant="destructive" className="mt-2">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {isSelf && (
        <p className="mt-3 text-sm text-muted-foreground">{t('selfActionDisabled')}</p>
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
              {t('setRole')}
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('setRoleTitle')}</DialogTitle>
              <DialogDescription>{t('setRoleDescription', { username: user.username })}</DialogDescription>
            </DialogHeader>
            <div className="grid gap-2 py-4">
              <Label htmlFor="new-role">{t('newRole')}</Label>
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
                {tCommon('cancel')}
              </Button>
              <Button onClick={handleSetRole} disabled={acting}>
                {acting ? tCommon('saving') : tCommon('save')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
        <AlertDialog
          open={deleteDialogOpen}
          onOpenChange={(open) => {
            setDeleteDialogOpen(open);
            if (!open) setDeleteConfirmInput('');
          }}
        >
          <AlertDialogTrigger asChild>
            <Button type="button" variant="destructive" disabled={acting || isSelf} className="bg-red-800 hover:bg-red-900">
              {t('deleteAccount')}
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t('deleteAccountTitle')}</AlertDialogTitle>
              <AlertDialogDescription>
                {t('deleteAccountDescription')}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <div className="space-y-2">
              <Label htmlFor="admin-delete-confirm" className="text-sm">
                {t('deleteAccountConfirmPrompt', { username: user.username })}
              </Label>
              <Input
                id="admin-delete-confirm"
                type="text"
                value={deleteConfirmInput}
                onChange={(e) => setDeleteConfirmInput(e.target.value)}
                placeholder={t('deleteAccountConfirmPlaceholder')}
                autoComplete="off"
                autoCapitalize="off"
                autoCorrect="off"
                spellCheck={false}
              />
            </div>
            <AlertDialogFooter>
              <AlertDialogCancel>{tCommon('cancel')}</AlertDialogCancel>
              <AlertDialogAction
                onClick={handleDelete}
                disabled={deleteConfirmInput !== user.username || acting}
                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              >
                {tCommon('delete')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </div>
  );
}
