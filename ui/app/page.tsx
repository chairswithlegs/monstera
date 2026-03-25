'use client';

import { getNodeInfo } from '@/lib/api/nodeinfo';
import { useAuth } from '@/hooks/useAuth';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import type { NodeInfoResponse } from '@/lib/api/nodeinfo';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { translateApiError } from '@/lib/i18n/errors';

export default function LandingPage() {
  const { token, loading: authLoading } = useAuth();
  const t = useTranslations('auth');
  const tErr = useTranslations('errors');
  const [nodeInfo, setNodeInfo] = useState<NodeInfoResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading) return;
    if (token) {
      window.location.replace('/home');
      return;
    }
    getNodeInfo()
      .then(setNodeInfo)
      .catch((e) => setError(translateApiError(tErr, e)))
      .finally(() => setLoading(false));
  }, [token, authLoading, tErr]);

  if (authLoading || token) return null;

  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center bg-background">
        <p className="text-muted-foreground">{tErr('failed_to_load')}</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-4 bg-background px-4">
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
        <Button asChild variant="link">
          <Link href="/login">{t('signIn')}</Link>
        </Button>
      </div>
    );
  }

  const { software, openRegistrations } = nodeInfo!;
  const serverName = (nodeInfo!.metadata.server_name as string) || software.name;
  const serverDescription = nodeInfo!.metadata.server_description as string | undefined;

  return (
    <div className="flex-1 bg-background">
      <div className="mx-auto max-w-2xl px-6 py-16">
        <h1 className="text-3xl font-bold text-foreground">{serverName}</h1>
        {serverDescription && (
          <p className="mt-4 text-base text-foreground">{serverDescription}</p>
        )}

        <div className="mt-10 flex flex-wrap items-center gap-4">
          <Button asChild>
            <Link href="/login">{t('signIn')}</Link>
          </Button>
          {openRegistrations ? (
            <Button asChild variant="outline">
              <Link href="/register">{t('register')}</Link>
            </Button>
          ) : (
            <div className="flex items-center gap-2">
              <Button variant="outline" disabled>
                {t('register')}
              </Button>
              <span className="text-sm text-muted-foreground">{t('registrationsClosed')}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
