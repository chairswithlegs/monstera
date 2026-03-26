import { describe, it, expect, vi } from 'vitest';
import { getPublicAccount } from './accounts';
import { ApiResponseError } from './errors';

vi.mock('@/lib/config', () => ({
  getConfig: vi.fn().mockResolvedValue({ server_url: 'https://api.example.com' }),
}));

describe('getPublicAccount', () => {
  it('returns parsed account on success', async () => {
    const mockAccount = {
      id: '01ABC',
      username: 'alice',
      acct: 'alice',
      display_name: 'Alice',
      note: '<p>Hello</p>',
      url: 'https://example.com/users/alice',
      avatar: 'https://example.com/avatar.png',
      header: 'https://example.com/header.png',
      locked: false,
      bot: false,
      created_at: '2024-01-01T00:00:00Z',
      followers_count: 10,
      following_count: 5,
      statuses_count: 42,
      fields: [],
    };

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(mockAccount),
    }));

    const result = await getPublicAccount('alice');
    expect(result).toEqual(mockAccount);
    expect(fetch).toHaveBeenCalledWith(
      'https://api.example.com/api/v1/accounts/lookup?acct=alice'
    );
  });

  it('throws ApiResponseError with code on 404', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: 'Record not found', code: 'not_found' }),
    }));

    const err = await getPublicAccount('nobody').catch((e) => e);
    expect(err).toBeInstanceOf(ApiResponseError);
    expect(err.code).toBe('not_found');
  });

  it('throws ApiResponseError on other non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'Internal server error', code: 'internal_error' }),
    }));

    const err = await getPublicAccount('alice').catch((e) => e);
    expect(err).toBeInstanceOf(ApiResponseError);
    expect(err.code).toBe('internal_error');
  });

  it('encodes special characters in the username', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({
        id: '01', username: 'user@remote.example', acct: 'user@remote.example',
        display_name: '', note: '', url: '', avatar: '', header: '',
        locked: false, bot: false, created_at: '',
        followers_count: 0, following_count: 0, statuses_count: 0, fields: [],
      }),
    }));

    await getPublicAccount('user@remote.example');
    expect(fetch).toHaveBeenCalledWith(
      'https://api.example.com/api/v1/accounts/lookup?acct=user%40remote.example'
    );
  });
});
