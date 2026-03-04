'use client';

import { getUser } from '@/lib/api/user';
import { useEffect, useState } from 'react';
import type { User } from '@/lib/api/user';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Card, CardContent } from '@/components/ui/card';

export default function HomePage() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getUser()
      .then(setUser)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="text-muted-foreground">Loading...</div>;
  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!user) return <div className="text-muted-foreground">Loading...</div>;

  return (
    <div>
      <h1 className="text-2xl font-semibold text-foreground">Home</h1>
      <p className="mt-2 text-muted-foreground">Welcome. Here is your account info.</p>
      <Card className="mt-4">
        <CardContent className="p-4">
          <pre className="overflow-auto">
            <code className="text-sm">{JSON.stringify(user, null, 2)}</code>
          </pre>
        </CardContent>
      </Card>
    </div>
  );
}
