'use client';
import { Suspense } from 'react';
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

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Authorize {params.appName}</CardTitle>
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
                is requesting access to your account.
              </>
            ) : (
              <>{params.appName} is requesting access to your account.</>
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <p className="mb-2 text-sm font-medium">Permissions requested:</p>
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
            submitLabel="Sign in and authorize"
          />
        </CardContent>
      </Card>
    </div>
  );
}

export default function AuthorizePage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-background">
          <Card className="w-full max-w-sm">
            <CardContent className="pt-6">
              <p className="text-muted-foreground">Loading...</p>
            </CardContent>
          </Card>
        </div>
      }
    >
      <AuthorizeContent />
    </Suspense>
  );
}
