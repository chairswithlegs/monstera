'use client';
import { useState, useEffect } from 'react';
import { getAccessToken } from '@/lib/auth/tokens';

interface AuthState {
  token: string | null;
  loading: boolean;
}

export function useAuth(): AuthState {
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // sessionStorage is only available client-side; defer setState to avoid synchronous cascading renders
    const token = getAccessToken();
    queueMicrotask(() => {
      setToken(token);
      setLoading(false);
    });
  }, []);

  return { token, loading };
}
