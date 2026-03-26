import type { ApiResponseError } from '@/lib/api/errors';

type TranslateFn = (key: string, values?: Record<string, string>) => string;

export function translateApiError(
  t: TranslateFn,
  err: unknown
): string {
  const apiErr = err as Partial<ApiResponseError>;
  if (apiErr?.code) {
    try {
      return t(apiErr.code, apiErr.params ?? {});
    } catch {
      // code not in messages, fall through
    }
  }
  if (apiErr?.message) return apiErr.message;
  return t('fallback');
}
