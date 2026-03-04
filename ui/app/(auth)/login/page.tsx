'use client';
import { Suspense, useState } from 'react';
import Link from 'next/link';
import { useLogin } from '@/hooks/useLogin';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';

function LoginContent() {
  const { loading, error, submitCredentials } = useLogin();

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Sign in</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <CredentialsForm onSubmit={submitCredentials} loading={loading} />

          <p className="mt-6 text-center text-sm text-muted-foreground">
            Don&apos;t have an account?{' '}
            <Button variant="link" size="sm" className="h-auto p-0" asChild>
              <Link href="/register">Register</Link>
            </Button>
          </p>
          <p className="mt-2 text-center text-sm text-muted-foreground">
            <Button variant="link" size="sm" className="h-auto p-0" asChild>
              <Link href="/">Back to home</Link>
            </Button>
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={
      <div className="flex min-h-screen items-center justify-center bg-background">
        <Card className="w-full max-w-sm">
          <CardContent className="pt-6">
            <p className="text-muted-foreground">Loading...</p>
          </CardContent>
        </Card>
      </div>
    }>
      <LoginContent />
    </Suspense>
  );
}

function CredentialsForm({
  onSubmit,
  loading,
}: {
  onSubmit: (username: string, password: string) => Promise<void>;
  loading: boolean;
}) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(username, password);
      }}
      className="flex flex-col gap-4"
    >
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="username">Username</Label>
        <Input
          id="username"
          type="text"
          autoComplete="username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          disabled={loading}
          required
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          disabled={loading}
          required
        />
      </div>
      <Button type="submit" disabled={loading} className="mt-2 w-full">
        {loading ? 'Signing in...' : 'Sign in'}
      </Button>
    </form>
  );
}
