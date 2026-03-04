'use client';

import { getInstances, getDomainBlocks, createDomainBlock, deleteDomainBlock, type KnownInstance, type DomainBlock } from '@/lib/api/admin';
import { useModeratorUser } from '@/contexts/moderator-user';
import { isAdmin } from '@/lib/api/user';
import { useCallback, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
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

export default function ModeratorFederationPage() {
  const currentUser = useModeratorUser();
  const [instances, setInstances] = useState<KnownInstance[]>([]);
  const [blocks, setBlocks] = useState<DomainBlock[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [newDomain, setNewDomain] = useState('');
  const [newReason, setNewReason] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(() => {
    getInstances({ limit: 50 })
      .then((r) => setInstances(r.instances))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load instances'));
    getDomainBlocks()
      .then((r) => setBlocks(r.domain_blocks))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load blocks'));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const addBlock = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDomain.trim()) return;
    setSubmitting(true);
    try {
      await createDomainBlock({ domain: newDomain.trim(), severity: 'silence', reason: newReason.trim() || undefined });
      setNewDomain('');
      setNewReason('');
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to add block');
    } finally {
      setSubmitting(false);
    }
  };

  const removeBlock = async (domain: string) => {
    try {
      await deleteDomainBlock(domain);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to remove block');
    }
  };

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }

  const showAdminActions = currentUser && isAdmin(currentUser);

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">Federation</h1>

      <section className="mt-8">
        <h2 className="text-lg font-medium text-foreground">Known instances</h2>
        <Card className="mt-4">
          <CardContent className="p-0">
            {instances.length === 0 ? (
              <EmptyState message="None yet." />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Domain</TableHead>
                    <TableHead>Accounts</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {instances.map((i) => (
                    <TableRow key={i.id}>
                      <TableCell>{i.domain}</TableCell>
                      <TableCell className="text-muted-foreground">{i.accounts_count}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </section>

      <section className="mt-8">
        <h2 className="text-lg font-medium text-foreground">Domain blocks</h2>
        {showAdminActions && (
          <form onSubmit={addBlock} className="mt-4 flex flex-wrap items-end gap-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="domain-block-domain">Domain</Label>
              <Input
                id="domain-block-domain"
                type="text"
                value={newDomain}
                onChange={(e) => setNewDomain(e.target.value)}
                placeholder="domain.example.com"
                className="min-w-[200px]"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="domain-block-reason">Reason (optional)</Label>
              <Input
                id="domain-block-reason"
                type="text"
                value={newReason}
                onChange={(e) => setNewReason(e.target.value)}
                placeholder="Reason (optional)"
                className="min-w-[200px]"
              />
            </div>
            <Button type="submit" disabled={submitting}>
              Add block
            </Button>
          </form>
        )}
        <Card className="mt-4">
          <CardContent className="p-0">
            {blocks.length === 0 ? (
              <EmptyState message="No domain blocks." />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Domain</TableHead>
                    <TableHead>Severity</TableHead>
                    {showAdminActions && <TableHead className="text-right">Actions</TableHead>}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {blocks.map((b) => (
                    <TableRow key={b.id}>
                      <TableCell>{b.domain}</TableCell>
                      <TableCell className="text-muted-foreground">{b.severity}</TableCell>
                      {showAdminActions && (
                        <TableCell className="text-right">
                          <Button type="button" variant="ghost" size="sm" onClick={() => removeBlock(b.domain)} className="text-destructive hover:text-destructive">
                            Remove
                          </Button>
                        </TableCell>
                      )}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
