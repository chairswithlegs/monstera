'use client';
import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { registerAccount } from '@/lib/api/accounts';
import { translateApiError } from '@/lib/i18n/errors';

interface RegisterState {
  loading: boolean;
  error: string | null;
  pending: boolean;
  success: boolean;
  submit: (
    username: string,
    email: string,
    password: string,
    reason?: string,
    inviteCode?: string
  ) => Promise<void>;
}

export function useRegister(): RegisterState {
  const t = useTranslations('errors');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [success, setSuccess] = useState(false);

  async function submit(
    username: string,
    email: string,
    password: string,
    reason?: string,
    inviteCode?: string
  ) {
    setLoading(true);
    setError(null);
    setPending(false);
    setSuccess(false);

    try {
      const result = await registerAccount({
        username,
        email,
        password,
        agreement: true,
        reason,
        invite_code: inviteCode,
      });

      if (result.pending) {
        setPending(true);
      } else {
        setSuccess(true);
      }
    } catch (err: unknown) {
      setError(translateApiError(t, err));
    } finally {
      setLoading(false);
    }
  }

  return { loading, error, pending, success, submit };
}
