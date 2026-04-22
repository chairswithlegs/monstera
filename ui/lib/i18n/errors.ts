import type { ApiResponseError } from '@/lib/api/errors';

type TranslateFn = (key: string, values?: Record<string, string>) => string;

// translateApiError picks the most specific translation available for an API
// error:
//
//  1. params.reason — the backend's structured reason (e.g.
//     "invalid_email_or_password", "unconfirmed", "cannot_perform_action_on_own_account").
//     Takes precedence because it's the narrowest label and multiple errors
//     share a code. Without this, every 401-with-reason collapsed to the
//     generic "unauthorized" message.
//  2. code — the HTTP-shape bucket ("unauthorized", "forbidden",
//     "validation_failed", etc.). Used as a fallback when there's no
//     translation for the specific reason, preserving existing behavior.
//  3. err.message — raw server-supplied text.
//  4. errors.fallback — last-resort generic string.
//
// A missing translation at any step cleanly falls through to the next one.
export function translateApiError(
  t: TranslateFn,
  err: unknown
): string {
  const apiErr = err as Partial<ApiResponseError>;
  const params = apiErr?.params ?? {};
  if (params.reason) {
    try {
      return t(params.reason, params);
    } catch {
      // reason not in messages, fall through
    }
  }
  if (apiErr?.code) {
    try {
      return t(apiErr.code, params);
    } catch {
      // code not in messages, fall through
    }
  }
  if (apiErr?.message) return apiErr.message;
  return t('fallback');
}
