'use client';

import { getRegistrations, approveRegistration, rejectRegistration, type PendingRegistration } from '@/lib/api/admin';
import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  Card,
  CardContent,
} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { EmptyState } from '@/components/empty-state';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

export default function ModeratorRegistrationsPage() {
  const [pending, setPending] = useState<PendingRegistration[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);
  const [rejectUserId, setRejectUserId] = useState<string | null>(null);
  const [rejectReason, setRejectReason] = useState('');

  const load = () => {
    getRegistrations()
      .then((r) => setPending(r.pending))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  };

  useEffect(() => {
    load();
  }, []);

  const approve = async (id: string) => {
    setActing(true);
    try {
      await approveRegistration(id);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed');
    } finally {
      setActing(false);
    }
  };

  const openRejectDialog = (userId: string) => {
    setRejectUserId(userId);
    setRejectReason('');
  };

  const reject = async () => {
    if (!rejectUserId) return;
    setActing(true);
    try {
      await rejectRegistration(rejectUserId, rejectReason);
      setRejectUserId(null);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed');
    } finally {
      setActing(false);
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
      <h1 className="text-2xl font-semibold text-foreground">Pending registrations</h1>
      <Card className="mt-6">
        <CardContent className="p-0">
          {pending.length === 0 ? (
            <EmptyState message="No pending registrations." />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Username</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Reason</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pending.map((p) => (
                  <TableRow key={p.user_id}>
                    <TableCell className="font-medium">@{p.username}</TableCell>
                    <TableCell className="text-muted-foreground">{p.email}</TableCell>
                    <TableCell className="text-muted-foreground">{p.registration_reason ?? '—'}</TableCell>
                    <TableCell className="text-right">
                      <Button
                        type="button"
                        variant="default"
                        size="sm"
                        disabled={acting}
                        onClick={() => approve(p.user_id)}
                        className="mr-2"
                      >
                        Approve
                      </Button>
                      <Button
                        type="button"
                        variant="destructive"
                        size="sm"
                        disabled={acting}
                        onClick={() => openRejectDialog(p.user_id)}
                      >
                        Reject
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={rejectUserId !== null} onOpenChange={(open) => !open && setRejectUserId(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reject registration</DialogTitle>
            <DialogDescription>Optionally provide a reason for the rejection.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-2 py-4">
            <Label htmlFor="reject-reason">Reason (optional)</Label>
            <Input
              id="reject-reason"
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              placeholder="Reason for rejection"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRejectUserId(null)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={reject} disabled={acting}>
              {acting ? 'Rejecting…' : 'Reject'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
