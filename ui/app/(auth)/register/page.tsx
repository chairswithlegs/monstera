'use client';

import Link from 'next/link';
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

export default function RegisterPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Create an account</CardTitle>
          <Alert variant="default" className="mt-2">
            <AlertDescription className="text-amber-600 dark:text-amber-500">
              Registration is not yet available. Check back later.
            </AlertDescription>
          </Alert>
        </CardHeader>
        <CardContent className="space-y-4">
          <form className="flex flex-col gap-4" onSubmit={(e) => e.preventDefault()}>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                autoComplete="email"
                disabled
                placeholder="Coming soon"
                className="opacity-60"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                type="text"
                autoComplete="username"
                disabled
                placeholder="Coming soon"
                className="opacity-60"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                autoComplete="new-password"
                disabled
                placeholder="Coming soon"
                className="opacity-60"
              />
            </div>
            <Button type="submit" disabled className="mt-2 w-full">
              Register
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
