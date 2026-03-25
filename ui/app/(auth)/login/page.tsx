'use client';
import { Suspense } from 'react';
import Link from 'next/link';
import { useTranslations } from 'next-intl';
import { useLogin } from '@/hooks/useLogin';
import { CredentialsForm } from '@/components/credentials-form';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';

function LoginContent() {
  const { loading, error, submitCredentials } = useLogin();
  const t = useTranslations('auth');

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>{t('loginTitle')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <CredentialsForm onSubmit={submitCredentials} loading={loading} />

          <p className="mt-6 text-center text-sm text-muted-foreground">
            {t('noAccount')}{' '}
            <Button variant="link" size="sm" className="h-auto p-0" asChild>
              <Link href="/register">{t('register')}</Link>
            </Button>
          </p>
          <p className="mt-2 text-center text-sm text-muted-foreground">
            <Button variant="link" size="sm" className="h-auto p-0" asChild>
              <Link href="/">{t('backToHome')}</Link>
            </Button>
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

function LoadingFallback() {
  const t = useTranslations('common');
  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardContent className="pt-6">
          <p className="text-muted-foreground">{t('loading')}</p>
        </CardContent>
      </Card>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={<LoadingFallback />}>
      <LoginContent />
    </Suspense>
  );
}
