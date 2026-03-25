'use client';
import { useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { getConfig } from '@/lib/config';
import { ApiResponseError } from '@/lib/api/errors';
import { translateApiError } from '@/lib/i18n/errors';

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
  const t = useTranslations('errors');
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
        const body = await response.json().catch(() => ({})) as { error?: string; code?: string; params?: Record<string, string> };
        throw new ApiResponseError(body.code ? body : { ...body, code: 'invalid_credentials' });
      }

      const data = (await response.json()) as { redirect_url: string };
      window.location.href = data.redirect_url;
    } catch (err: unknown) {
      setError(translateApiError(t, err));
    } finally {
      setLoading(false);
    }
  }

  return { params, scopes, loading, error, submitCredentials };
}
