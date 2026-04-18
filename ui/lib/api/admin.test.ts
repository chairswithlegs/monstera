import { describe, it, expect, vi, beforeEach } from 'vitest';
import { deleteUser } from './admin';
import { ApiResponseError } from './errors';

vi.mock('@/lib/config', () => ({
  getConfig: vi.fn().mockResolvedValue({ server_url: 'https://api.example.com' }),
}));

const authFetchMock = vi.fn();
vi.mock('./client', () => ({
  authFetch: (...args: unknown[]) => authFetchMock(...args),
}));

describe('deleteUser', () => {
  beforeEach(() => {
    authFetchMock.mockReset();
  });

  it('sends DELETE without force by default (soft-delete)', async () => {
    authFetchMock.mockResolvedValue({ ok: true, status: 204 });

    await deleteUser('01ACCOUNT');

    const [url, init] = authFetchMock.mock.calls[0];
    expect(url).toBe('https://api.example.com/monstera/api/v1/admin/users/01ACCOUNT');
    expect(init).toMatchObject({ method: 'DELETE' });
  });

  it('appends ?force=true when force is set', async () => {
    authFetchMock.mockResolvedValue({ ok: true, status: 204 });

    await deleteUser('01ACCOUNT', { force: true });

    const [url] = authFetchMock.mock.calls[0];
    expect(url).toBe('https://api.example.com/monstera/api/v1/admin/users/01ACCOUNT?force=true');
  });

  it('percent-encodes ids with special characters', async () => {
    authFetchMock.mockResolvedValue({ ok: true, status: 204 });

    await deleteUser('weird id/with slash');

    const [url] = authFetchMock.mock.calls[0];
    expect(url).toBe('https://api.example.com/monstera/api/v1/admin/users/weird%20id%2Fwith%20slash');
  });

  it('throws ApiResponseError on non-ok response', async () => {
    authFetchMock.mockResolvedValue({
      ok: false,
      status: 422,
      json: () =>
        Promise.resolve({
          error: 'Cannot perform this action on your own account',
          code: 'cannot_perform_action_on_own_account',
        }),
    });

    const err = await deleteUser('01ACCOUNT').catch((e) => e);
    expect(err).toBeInstanceOf(ApiResponseError);
    expect(err.code).toBe('cannot_perform_action_on_own_account');
  });
});
