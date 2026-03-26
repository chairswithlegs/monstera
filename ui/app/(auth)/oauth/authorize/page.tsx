'use client';
import { Suspense } from 'react';
import { useTranslations } from 'next-intl';
import { useAuthorize } from '@/hooks/useAuthorize';
import { getScopeLabel } from '@/lib/auth/scope-labels';
import { CredentialsForm } from '@/components/credentials-form';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';

function AuthorizeContent() {
  const { params, scopes, loading, error, submitCredentials } = useAuthorize();
  const t = useTranslations('auth');

  return (
    <div className="flex flex-1 items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>{t('authorizeTitle', { appName: params.appName })}</CardTitle>
          <CardDescription>
            {params.appWebsite ? (
              <>
                <a
                  href={params.appWebsite}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="underline"
                >
                  {params.appName}
                </a>{' '}
                {t('requestsAccountAccess')}
              </>
            ) : (
              <>{t('authorizeDescription', { appName: params.appName })}</>
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <p className="mb-2 text-sm font-medium">{t('permissionsRequested')}</p>
            <ul className="list-inside list-disc space-y-1 text-sm text-muted-foreground">
              {scopes.map((scope) => (
                <li key={scope}>{getScopeLabel(scope)}</li>
              ))}
            </ul>
          </div>

          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <CredentialsForm
            onSubmit={submitCredentials}
            loading={loading}
            submitLabel={t('signInAndAuthorize')}
          />
        </CardContent>
      </Card>
    </div>
  );
}

function LoadingFallback() {
  const t = useTranslations('common');
  return (
    <div className="flex flex-1 items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardContent className="pt-6">
          <p className="text-muted-foreground">{t('loading')}</p>
        </CardContent>
      </Card>
    </div>
  );
}

export default function AuthorizePage() {
  return (
    <Suspense fallback={<LoadingFallback />}>
      <AuthorizeContent />
    </Suspense>
  );
}
