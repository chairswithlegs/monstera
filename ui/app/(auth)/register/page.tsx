'use client';

import Link from 'next/link';
import { useState, useEffect } from 'react';
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useRegister } from '@/hooks/useRegister';
import { getNodeInfo } from '@/lib/api/nodeinfo';

export default function RegisterPage() {
  const { loading, error, pending, success, submit } = useRegister();
  const [email, setEmail] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [reason, setReason] = useState('');
  const [inviteCode, setInviteCode] = useState('');
  const [registrationMode, setRegistrationMode] = useState<string | null>(null);
  const [serverRules, setServerRules] = useState<string[]>([]);
  const [rulesAccepted, setRulesAccepted] = useState(false);

  useEffect(() => {
    getNodeInfo()
      .then((info) => {
        setRegistrationMode((info.metadata.registration_mode as string) ?? 'open');
        const rules = info.metadata.server_rules as string[] | undefined;
        if (rules && rules.length > 0) {
          setServerRules(rules);
        }
      })
      .catch(() => setRegistrationMode('open'));
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await submit(username, email, password, reason || undefined, inviteCode || undefined);
  };

  if (pending) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <Card className="w-full max-w-sm">
          <CardHeader>
            <CardTitle>Registration submitted</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              Your registration is pending approval. You will be notified once your account is approved.
            </p>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (success) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <Card className="w-full max-w-sm">
          <CardHeader>
            <CardTitle>Registration successful!</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">
              Your account has been created.{' '}
              <Button variant="link" size="sm" className="h-auto p-0" asChild>
                <Link href="/login">Sign in</Link>
              </Button>
            </p>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Dialog open={serverRules.length > 0 && !rulesAccepted}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Server rules</DialogTitle>
            <DialogDescription>
              Please read and agree to these rules before registering.
            </DialogDescription>
          </DialogHeader>
          <ol className="list-decimal pl-5 space-y-2 text-sm">
            {serverRules.map((rule, i) => (
              <li key={i}>{rule}</li>
            ))}
          </ol>
          <DialogFooter>
            <Button onClick={() => setRulesAccepted(true)}>
              I agree to these rules
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Create an account</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                autoComplete="email"
                disabled={loading}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                type="text"
                autoComplete="username"
                disabled={loading}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                autoComplete="new-password"
                disabled={loading}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="reason">Why do you want to join? (optional)</Label>
              <textarea
                id="reason"
                disabled={loading}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                rows={3}
                className="flex min-h-[60px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
              />
            </div>
            {registrationMode === 'invite' && (
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="invite-code">Invite code</Label>
                <Input
                  id="invite-code"
                  type="text"
                  disabled={loading}
                  value={inviteCode}
                  onChange={(e) => setInviteCode(e.target.value)}
                  required
                />
              </div>
            )}
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            <Button type="submit" disabled={loading} className="mt-2 w-full">
              {loading ? 'Registering…' : 'Register'}
            </Button>
          </form>

          <p className="mt-6 text-center text-sm text-muted-foreground">
            Already have an account?{' '}
            <Button variant="link" size="sm" className="h-auto p-0" asChild>
              <Link href="/login">Sign in</Link>
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
