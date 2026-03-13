'use client';

import { getFilters, createFilter, deleteFilter, type ServerFilter } from '@/lib/api/admin';
import { useCallback, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
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

export default function ModeratorContentPage() {
  const [filters, setFilters] = useState<ServerFilter[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [newPhrase, setNewPhrase] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(() => {
    getFilters()
      .then((r) => setFilters(r.filters))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const addFilter = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newPhrase.trim()) return;
    setSubmitting(true);
    try {
      await createFilter({ phrase: newPhrase.trim(), scope: 'all', action: 'hide' });
      setNewPhrase('');
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to add filter');
    } finally {
      setSubmitting(false);
    }
  };

  const removeFilter = async (id: string) => {
    try {
      await deleteFilter(id);
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
      <h1 className="text-2xl font-semibold text-foreground">Content</h1>
      <p className="mt-1 text-sm text-muted-foreground">Server-side keyword filters.</p>

      <section className="mt-8">
        <h2 className="text-lg font-medium text-foreground">Server filters</h2>
        <form onSubmit={addFilter} className="mt-4 flex gap-2">
          <Input
            type="text"
            value={newPhrase}
            onChange={(e) => setNewPhrase(e.target.value)}
            placeholder="Keyword or phrase"
            className="min-w-[200px]"
          />
          <Button type="submit" disabled={submitting}>
            Add filter
          </Button>
        </form>
        <Card className="mt-4">
          <CardContent>
            {filters.length === 0 ? (
              <EmptyState message="No filters." />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Phrase</TableHead>
                    <TableHead>Scope</TableHead>
                    <TableHead>Action</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filters.map((f) => (
                    <TableRow key={f.id}>
                      <TableCell>{f.phrase}</TableCell>
                      <TableCell className="text-muted-foreground">{f.scope}</TableCell>
                      <TableCell className="text-muted-foreground">{f.action}</TableCell>
                      <TableCell className="text-right">
                        <Button type="button" variant="ghost" size="sm" onClick={() => removeFilter(f.id)} className="text-destructive hover:text-destructive">
                          Delete
                        </Button>
                      </TableCell>
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
