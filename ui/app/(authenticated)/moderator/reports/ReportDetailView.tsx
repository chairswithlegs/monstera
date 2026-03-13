'use client';

import { getReport, resolveReport, type Report } from '@/lib/api/admin';
import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
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

export default function ReportDetailView({ id }: Props) {
  const [report, setReport] = useState<Report | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);
  const [resolveOpen, setResolveOpen] = useState(false);
  const [resolutionNote, setResolutionNote] = useState('');

  const load = useCallback(() => {
    if (!id) return;
    getReport(id)
      .then(setReport)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const resolve = async () => {
    setActing(true);
    try {
      await resolveReport(id, resolutionNote);
      setResolveOpen(false);
      setResolutionNote('');
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed');
    } finally {
      setActing(false);
    }
  };

  if (error && !report) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!report) return <p className="text-muted-foreground">Loading…</p>;

  return (
    <div>
      <p className="mb-4">
        <Button variant="link" size="sm" className="h-auto p-0" asChild>
          <Link href="/moderator/reports">← Back to reports</Link>
        </Button>
      </p>
      <h1 className="text-2xl font-semibold text-foreground">Report {report.id.slice(0, 8)}…</h1>
      <dl className="mt-6 grid gap-2 sm:grid-cols-2">
        <dt className="text-sm font-medium text-muted-foreground">Category</dt>
        <dd className="text-sm text-foreground">{report.category}</dd>
        <dt className="text-sm font-medium text-muted-foreground">State</dt>
        <dd className="text-sm text-foreground">{report.state}</dd>
        <dt className="text-sm font-medium text-muted-foreground">Reporter account</dt>
        <dd className="font-mono text-sm text-foreground">{report.account_id}</dd>
        <dt className="text-sm font-medium text-muted-foreground">Target account</dt>
        <dd className="font-mono text-sm text-foreground">{report.target_id}</dd>
        {report.comment && (
          <>
            <dt className="text-sm font-medium text-muted-foreground">Comment</dt>
            <dd className="text-sm text-foreground">{report.comment}</dd>
          </>
        )}
        {report.action_taken && (
          <>
            <dt className="text-sm font-medium text-muted-foreground">Action taken</dt>
            <dd className="text-sm text-foreground">{report.action_taken}</dd>
          </>
        )}
      </dl>
      {error && (
        <Alert variant="destructive" className="mt-2">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {report.state === 'open' && (
        <div className="mt-6 flex gap-2">
          <Dialog open={resolveOpen} onOpenChange={setResolveOpen}>
            <DialogTrigger asChild>
              <Button type="button" disabled={acting}>
                Resolve report
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Resolve report</DialogTitle>
                <DialogDescription>Add a resolution note (optional).</DialogDescription>
              </DialogHeader>
              <div className="grid gap-2 py-4">
                <Label htmlFor="resolution-note">Resolution note</Label>
                <Input
                  id="resolution-note"
                  value={resolutionNote}
                  onChange={(e) => setResolutionNote(e.target.value)}
                  placeholder="What action was taken?"
                />
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setResolveOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={resolve} disabled={acting}>
                  {acting ? 'Resolving…' : 'Resolve'}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      )}
    </div>
  );
}
