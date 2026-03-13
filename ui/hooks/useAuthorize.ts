'use client';
import { useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { getConfig } from '@/lib/config';

interface AuthorizeParams {
  clientId: string;
  redirectUri: string;
  scope: string;
  codeChallenge: string;
  codeChallengeMethod: string;
  state: string | null;
  appName: string;
  appWebsite: string | null;
}

interface AuthorizeState {
  params: AuthorizeParams;
  scopes: string[];
  loading: boolean;
  error: string | null;
  submitCredentials: (email: string, password: string) => Promise<void>;
}

export function useAuthorize(): AuthorizeState {
  const searchParams = useSearchParams();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const params: AuthorizeParams = {
    clientId: searchParams.get('client_id') ?? '',
    redirectUri: searchParams.get('redirect_uri') ?? '',
    scope: searchParams.get('scope') ?? 'read',
    codeChallenge: searchParams.get('code_challenge') ?? '',
    codeChallengeMethod: searchParams.get('code_challenge_method') ?? '',
    state: searchParams.get('state'),
    appName: searchParams.get('app_name') ?? 'Unknown application',
    appWebsite: searchParams.get('app_website'),
  };

  const scopes = params.scope.split(' ').filter(Boolean);

  async function submitCredentials(email: string, password: string) {
    const config = await getConfig();
    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`${config.server_url}/oauth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email,
          password,
          client_id: params.clientId,
          redirect_uri: params.redirectUri,
          scope: params.scope,
          code_challenge: params.codeChallenge,
          code_challenge_method: params.codeChallengeMethod,
          ...(params.state && { state: params.state }),
        }),
      });

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(
          (body as { error?: string }).error ?? 'Invalid credentials'
        );
      }

      const data = (await response.json()) as { redirect_url: string };
      window.location.href = data.redirect_url;
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setLoading(false);
    }
  }

  return { params, scopes, loading, error, submitCredentials };
}
