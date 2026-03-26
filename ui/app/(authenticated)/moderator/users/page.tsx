'use client';

import { getUsers, type AdminUser } from '@/lib/api/admin';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import UserDetailView from './UserDetailView';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
} from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { EmptyState } from '@/components/empty-state';
import { translateApiError } from '@/lib/i18n/errors';

function UsersList() {
  const t = useTranslations('moderator');
  const tCommon = useTranslations('common');
  const tErr = useTranslations('errors');
  const tEmpty = useTranslations('empty');
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getUsers({ limit: 50 })
      .then((r) => setUsers(r.users))
      .catch((e) => setError(translateApiError(tErr, e)));
  }, [tErr]);

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">{t('usersTitle')}</h1>
      <Card className="mt-6">
        <CardContent>
          {users.length === 0 ? (
            <EmptyState message={tEmpty('noUsers')} />
          ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('usersColUsername')}</TableHead>
                <TableHead>{t('usersColEmail')}</TableHead>
                <TableHead>{t('usersColRole')}</TableHead>
                <TableHead>{t('usersColStatus')}</TableHead>
                <TableHead className="text-right">{t('usersColActions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((u) => (
                <TableRow key={u.id}>
                  <TableCell>
                    <Button variant="link" size="sm" className="h-auto p-0 font-medium" asChild>
                      <Link href={`/moderator/users?id=${encodeURIComponent(u.account_id)}`}>
                        @{u.username}
                      </Link>
                    </Button>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{u.email}</TableCell>
                  <TableCell className="text-muted-foreground">{u.role}</TableCell>
                  <TableCell>
                    {u.suspended && <Badge variant="destructive">{t('badgeSuspended')}</Badge>}
                    {u.silenced && !u.suspended && <Badge variant="secondary">{t('badgeSilenced')}</Badge>}
                    {!u.suspended && !u.silenced && <span className="text-muted-foreground">—</span>}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button variant="link" size="sm" asChild>
                      <Link href={`/moderator/users?id=${encodeURIComponent(u.account_id)}`}>
                        {tCommon('view')}
                      </Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function LoadingFallback() {
  const tCommon = useTranslations('common');
  return <p className="text-muted-foreground">{tCommon('loading')}</p>;
}

export default function ModeratorUsersPage() {
  const searchParams = useSearchParams();
  const id = searchParams.get('id');

  if (id) {
    return <UserDetailView id={id} />;
  }

  return (
    <Suspense fallback={<LoadingFallback />}>
      <UsersList />
    </Suspense>
  );
}
