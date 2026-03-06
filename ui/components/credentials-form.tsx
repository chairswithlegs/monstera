'use client';
import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';

interface CredentialsFormProps {
  onSubmit: (email: string, password: string) => Promise<void>;
  loading: boolean;
  submitLabel?: string;
}

export function CredentialsForm({
  onSubmit,
  loading,
  submitLabel = 'Sign in',
}: CredentialsFormProps) {
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
        {loading ? `${submitLabel}...` : submitLabel}
      </Button>
    </form>
  );
}
