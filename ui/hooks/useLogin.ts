'use client';
import { useState } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { generateCodeVerifier, generateCodeChallenge } from '@/lib/auth/pkce';
import { storeTokens } from '@/lib/auth/tokens';
import { AUTH_CLIENT_ID, AUTH_SCOPES, getMonsteraServerUrl, getDashboardRedirectUri } from '@/lib/config';

type Stage = 'credentials';

interface LoginState {
  stage: Stage;
  loading: boolean;
  error: string | null;
  submitCredentials: (username: string, password: string) => Promise<void>;
}

export function useLogin(): LoginState {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [stage, setStage] = useState<Stage>('credentials');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const MONSTERA_SERVER = getMonsteraServerUrl();
  const DASHBOARD_REDIRECT_URI = getDashboardRedirectUri();

  const params = {
    clientId: searchParams.get('client_id'),
    redirectUri: searchParams.get('redirect_uri'),
    codeChallenge: searchParams.get('code_challenge'),
    codeChallengeMethod: searchParams.get('code_challenge_method'),
    state: searchParams.get('state'),
  };

  // Check the client ID to determine if we are creating a login request for a third-party client
  // or for the internal UI client
  const isThirdParty = params.clientId && params.clientId !== AUTH_CLIENT_ID;

  async function handleCode(code: string) {
    if (isThirdParty) {
      // Third-party flow: redirect back to the client app with the code
      const redirectParams = new URLSearchParams({ code });
      if (params.state) redirectParams.set('state', params.state);
      window.location.href = `${params.redirectUri}?${params}`;
    } else {
      // Internal flow: exchange code for tokens ourselves
      const verifier = sessionStorage.getItem('pkce_verifier');
      if (!verifier) throw new Error('Missing PKCE verifier');
      sessionStorage.removeItem('pkce_verifier');

      const response = await fetch(`${MONSTERA_SERVER}/oauth/token`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({
          grant_type: 'authorization_code',
          code,
          redirect_uri: DASHBOARD_REDIRECT_URI,
          client_id: AUTH_CLIENT_ID,
          code_verifier: verifier,
        }),
      });

      if (!response.ok) throw new Error('Token exchange failed');
      const { access_token, refresh_token } = await response.json();
      storeTokens(access_token, refresh_token);
      router.replace('/');
    }
  }

  async function submitCredentials(email: string, password: string) {
    setLoading(true);
    setError(null);

    try {
      // For internal flow, we generate PKCE here and store the verifier
      // For third-party flow, the client app already sent code_challenge in the URL
      let codeChallenge = params.codeChallenge;
      if (!isThirdParty) {
        const verifier = generateCodeVerifier();
        codeChallenge = await generateCodeChallenge(verifier);
        sessionStorage.setItem('pkce_verifier', verifier);
      }

      const response = await fetch(`${MONSTERA_SERVER}/oauth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email,
          password,
          client_id: params.clientId ?? AUTH_CLIENT_ID,
          redirect_uri: params.redirectUri ?? DASHBOARD_REDIRECT_URI,
          scope: isThirdParty ? undefined : AUTH_SCOPES,
          code_challenge: codeChallenge,
          code_challenge_method: params.codeChallengeMethod ?? 'S256',
        }),
      });

      if (!response.ok) {
        const { error } = await response.json();
        throw new Error(error ?? 'Invalid credentials');
      }

      const data = await response.json();
      await handleCode(data.code);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  return { stage, loading, error, submitCredentials };
}
