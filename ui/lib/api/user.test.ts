import { describe, it, expect, vi, beforeEach } from 'vitest';
import { deleteAccount } from './user';
import { ApiResponseError } from './errors';

vi.mock('@/lib/config', () => ({
  getConfig: vi.fn().mockResolvedValue({ server_url: 'https://api.example.com' }),
}));

// Replace authFetch so we don't need real tokens / the 401 refresh dance.
// The actual behavior under test is how deleteAccount maps responses to
// ApiResponseError; the transport + auth layer is covered elsewhere.
const authFetchMock = vi.fn();
vi.mock('./client', () => ({
  authFetch: (...args: unknown[]) => authFetchMock(...args),
}));

describe('deleteAccount', () => {
  beforeEach(() => {
    authFetchMock.mockReset();
  });

  it('resolves when the server returns 204', async () => {
    authFetchMock.mockResolvedValue({ ok: true, status: 204 });

    await expect(deleteAccount({ current_password: 'hunter2' })).resolves.toBeUndefined();

    const [url, init] = authFetchMock.mock.calls[0];
    expect(url).toBe('https://api.example.com/monstera/api/v1/user');
    expect(init).toMatchObject({
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ current_password: 'hunter2' }),
    });
  });

  it('throws ApiResponseError with code forbidden on 403 (wrong password)', async () => {
    authFetchMock.mockResolvedValue({
      ok: false,
      status: 403,
      json: () => Promise.resolve({ error: 'Forbidden', code: 'forbidden' }),
    });

    const err = await deleteAccount({ current_password: 'nope' }).catch((e) => e);
    expect(err).toBeInstanceOf(ApiResponseError);
    expect(err.code).toBe('forbidden');
  });

  it('throws ApiResponseError with code deletion_already_requested on 409', async () => {
    authFetchMock.mockResolvedValue({
      ok: false,
      status: 409,
      json: () => Promise.resolve({
        error: 'deletion already requested',
        code: 'deletion_already_requested',
      }),
    });

    const err = await deleteAccount({ current_password: 'hunter2' }).catch((e) => e);
    expect(err).toBeInstanceOf(ApiResponseError);
    expect(err.code).toBe('deletion_already_requested');
  });

  it('throws ApiResponseError with validation code on 422 (empty password)', async () => {
    authFetchMock.mockResolvedValue({
      ok: false,
      status: 422,
      json: () => Promise.resolve({
        error: 'current_password is required',
        code: 'validation_failed',
        params: { field: 'current_password' },
      }),
    });

    const err = await deleteAccount({ current_password: '' }).catch((e) => e);
    expect(err).toBeInstanceOf(ApiResponseError);
    expect(err.code).toBe('validation_failed');
  });
});
