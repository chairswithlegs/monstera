'use client';

import { getNodeInfo } from '@/lib/api/nodeinfo';
import { useAuth } from '@/hooks/useAuth';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import type { NodeInfoResponse } from '@/lib/api/nodeinfo';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { StatsCard } from '@/components/stats-card';

export default function LandingPage() {
  const { token, loading: authLoading } = useAuth();
  const [nodeInfo, setNodeInfo] = useState<NodeInfoResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading) return;
    if (token) {
      window.location.replace('/home');
      return;
    }
    getNodeInfo()
      .then(setNodeInfo)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false));
  }, [token, authLoading]);

  if (authLoading || token) return null;

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-background px-4">
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
        <Button asChild variant="link">
          <Link href="/login">Sign in</Link>
        </Button>
      </div>
    );
  }

  const { software, usage, openRegistrations } = nodeInfo!;

  return (
    <div className="min-h-screen bg-background">
      <div className="mx-auto max-w-2xl px-6 py-16">
        <h1 className="text-3xl font-bold text-foreground">{software.name}</h1>
        <p className="mt-2 text-muted-foreground">
          A federated server running {software.name} {software.version}.
        </p>

        <div className="mt-8 grid gap-4 sm:grid-cols-2">
          <StatsCard label="Users" value={usage.users.total.toLocaleString()} />
          <StatsCard label="Local posts" value={usage.localPosts.toLocaleString()} />
        </div>

        <p className="mt-4 text-sm text-muted-foreground">
          Registrations are {openRegistrations ? 'open' : 'closed'}.
        </p>

        <div className="mt-10 flex flex-wrap gap-4">
          <Button asChild>
            <Link href="/login">Sign in</Link>
          </Button>
          {openRegistrations && (
            <Button asChild variant="outline">
              <Link href="/register">Register</Link>
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
