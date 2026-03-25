'use client';
import { useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { generateCodeVerifier, generateCodeChallenge } from '@/lib/auth/pkce';
import { storeTokens } from '@/lib/auth/tokens';
import { getConfig } from '@/lib/config';
import { MONSTERA_UI_OAUTH_APPLICATION_ID, MONSTERA_UI_OAUTH_SCOPES } from '@/lib/auth/oauth';

const uiLoginRedirectPath = '/home';

function getUILoginRedirectUri(): string {
  if (typeof window === 'undefined') return '';
  return `${window.location.origin}${uiLoginRedirectPath}`;
}

interface LoginState {
  loading: boolean;
  error: string | null;
  submitCredentials: (email: string, password: string) => Promise<void>;
}

export function useLogin(): LoginState {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submitCredentials(email: string, password: string) {
    const config = await getConfig();
    setLoading(true);
    setError(null);

    try {
      const verifier = generateCodeVerifier();
      const codeChallenge = await generateCodeChallenge(verifier);
      sessionStorage.setItem('pkce_verifier', verifier);

      const response = await fetch(`${config.server_url}/oauth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email,
          password,
          client_id: MONSTERA_UI_OAUTH_APPLICATION_ID,
          redirect_uri: getUILoginRedirectUri(),
          scope: MONSTERA_UI_OAUTH_SCOPES,
          code_challenge: codeChallenge,
          code_challenge_method: 'S256',
        }),
      });

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(
          (body as { error?: string }).error ?? 'Invalid credentials'
        );
      }

      const data = (await response.json()) as { redirect_url: string };
      const url = new URL(data.redirect_url);
      const code = url.searchParams.get('code');
      if (!code) throw new Error('Missing code in redirect URL');

      await exchangeCodeForTokens(code);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setLoading(false);
    }
  }

  async function exchangeCodeForTokens(code: string) {
    const config = await getConfig();
    const verifier = sessionStorage.getItem('pkce_verifier');
    if (!verifier) throw new Error('Missing PKCE verifier');
    sessionStorage.removeItem('pkce_verifier');

    const response = await fetch(`${config.server_url}/oauth/token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        grant_type: 'authorization_code',
        code,
        redirect_uri: getUILoginRedirectUri(),
        client_id: MONSTERA_UI_OAUTH_APPLICATION_ID,
        code_verifier: verifier,
      }),
    });

    if (!response.ok) throw new Error('Token exchange failed');
    const { access_token, refresh_token } = await response.json();
    storeTokens(access_token, refresh_token);
    const redirectTo = searchParams.get('redirect');
    router.replace(redirectTo && redirectTo.startsWith('/') ? redirectTo : uiLoginRedirectPath);
  }

  return { loading, error, submitCredentials };
}
