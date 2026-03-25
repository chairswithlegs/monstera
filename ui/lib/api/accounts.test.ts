import { describe, it, expect, vi } from 'vitest';
import { getPublicAccount } from './accounts';

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

  it('throws "Profile not found" on 404', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
    }));

    await expect(getPublicAccount('nobody')).rejects.toThrow('Profile not found');
  });

  it('throws "Failed to load profile" on other non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
    }));

    await expect(getPublicAccount('alice')).rejects.toThrow('Failed to load profile');
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
