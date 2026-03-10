'use client';

import { getUsers, type AdminUser } from '@/lib/api/admin';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useState } from 'react';
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

function UsersList() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getUsers({ limit: 50 })
      .then((r) => setUsers(r.users))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, []);

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">Users</h1>
      <Card className="mt-6">
        <CardContent>
          {users.length === 0 ? (
            <EmptyState message="No users." />
          ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Username</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
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
                    {u.suspended && <Badge variant="destructive">Suspended</Badge>}
                    {u.silenced && !u.suspended && <Badge variant="secondary">Silenced</Badge>}
                    {!u.suspended && !u.silenced && <span className="text-muted-foreground">—</span>}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button variant="link" size="sm" asChild>
                      <Link href={`/moderator/users?id=${encodeURIComponent(u.account_id)}`}>
                        View
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

export default function ModeratorUsersPage() {
  const searchParams = useSearchParams();
  const id = searchParams.get('id');

  if (id) {
    return <UserDetailView id={id} />;
  }

  return (
    <Suspense fallback={<p className="text-muted-foreground">Loading…</p>}>
      <UsersList />
    </Suspense>
  );
}
