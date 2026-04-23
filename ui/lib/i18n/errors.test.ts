import { describe, it, expect } from 'vitest';
import { translateApiError } from './errors';
import { ApiResponseError } from '@/lib/api/errors';

// Minimal translator stub that simulates next-intl behavior: return the value
// for known keys, throw for unknown ones. This matches how translateApiError
// detects "no translation" via try/catch.
function makeT(messages: Record<string, string>) {
  return (key: string, values?: Record<string, string>) => {
    if (!(key in messages)) {
      throw new Error(`unknown key: ${key}`);
    }
    let out = messages[key];
    for (const [k, v] of Object.entries(values ?? {})) {
      out = out.replaceAll(`{${k}}`, v);
    }
    return out;
  };
}

describe('translateApiError', () => {
  it('prefers params.reason over code', () => {
    // Regression: before this change, every 401 with a specific reason
    // (invalid credentials, outside-of-scopes, etc.) collapsed to the
    // generic "unauthorized" string because only code was consulted.
    const t = makeT({
      unauthorized: 'You must be signed in.',
      invalid_email_or_password: 'Invalid email or password.',
    });
    const err = new ApiResponseError({
      error: 'unauthorized',
      code: 'unauthorized',
      params: { reason: 'invalid_email_or_password' },
    });
    expect(translateApiError(t, err)).toBe('Invalid email or password.');
  });

  it('falls back to code when reason has no translation', () => {
    const t = makeT({
      unauthorized: 'You must be signed in.',
    });
    const err = new ApiResponseError({
      error: 'unauthorized',
      code: 'unauthorized',
      params: { reason: 'some_brand_new_reason' },
    });
    expect(translateApiError(t, err)).toBe('You must be signed in.');
  });

  it('falls back to err.message when neither reason nor code translates', () => {
    const t = makeT({ fallback: 'An error occurred.' });
    const err = new ApiResponseError({
      error: 'server said something specific',
      code: 'unknown_code',
    });
    expect(translateApiError(t, err)).toBe('server said something specific');
  });

  it('passes params into the translator so {field} interpolation works', () => {
    const t = makeT({
      validation_failed: 'Validation failed: {field}.',
    });
    const err = new ApiResponseError({
      error: 'validation_failed',
      code: 'validation_failed',
      params: { field: 'email' },
    });
    expect(translateApiError(t, err)).toBe('Validation failed: email.');
  });

  it('uses code when params.reason is absent', () => {
    const t = makeT({
      not_found: 'Not found.',
    });
    const err = new ApiResponseError({
      error: 'not_found',
      code: 'not_found',
    });
    expect(translateApiError(t, err)).toBe('Not found.');
  });

  it('returns fallback when everything misses', () => {
    const t = makeT({ fallback: 'An error occurred.' });
    expect(translateApiError(t, new Error('bare error'))).toBe('bare error');
    expect(translateApiError(t, {})).toBe('An error occurred.');
  });
});
