'use client';

import { getDashboard } from '@/lib/api/admin';
import { useEffect, useState } from 'react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  Card,
  CardContent,
  CardHeader,
} from '@/components/ui/card';

export default function ModeratorDashboardPage() {
  const [data, setData] = useState<Awaited<ReturnType<typeof getDashboard>> | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getDashboard()
      .then(setData)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'));
  }, []);

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!data) return <p className="text-muted-foreground">Loading…</p>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">Dashboard</h1>
      <div className="mt-6 grid gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <p className="text-sm font-medium text-muted-foreground">Local users</p>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold text-foreground">{data.local_users_count}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <p className="text-sm font-medium text-muted-foreground">Local posts</p>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold text-foreground">{data.local_statuses_count}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <p className="text-sm font-medium text-muted-foreground">Open reports</p>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold text-foreground">{data.open_reports_count}</p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
