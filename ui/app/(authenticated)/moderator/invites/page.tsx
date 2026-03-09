'use client';

import { getInvites, createInvite, deleteInvite, type Invite } from '@/lib/api/admin';
import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
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

export default function ModeratorInvitesPage() {
  const [invites, setInvites] = useState<Invite[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const load = () => {
    getInvites()
      .then((r) => setInvites(r.invites))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  };

  useEffect(() => {
    load();
  }, []);

  const create = async () => {
    setCreating(true);
    try {
      await createInvite({});
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create');
    } finally {
      setCreating(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await deleteInvite(id);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete');
    }
  };

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">Invites</h1>
      <div className="mt-4">
        <Button type="button" disabled={creating} onClick={create}>
          Create invite
        </Button>
      </div>
      <Card className="mt-6">
        <CardContent className="p-0">
          {invites.length === 0 ? (
            <EmptyState message="No invites yet." />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Code</TableHead>
                  <TableHead>Uses</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Expires</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {invites.map((inv) => (
                  <TableRow key={inv.id}>
                    <TableCell className="font-mono">{inv.code}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {inv.uses}
                      {inv.max_uses != null ? ` / ${inv.max_uses}` : ''}
                    </TableCell>
                    <TableCell className="text-muted-foreground">{new Date(inv.created_at).toLocaleString()}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {inv.expires_at ? new Date(inv.expires_at).toLocaleDateString() : 'Never'}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button type="button" variant="ghost" size="sm" onClick={() => remove(inv.id)} className="text-destructive hover:text-destructive">
                        Revoke
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
