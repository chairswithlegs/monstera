import { getConfig } from '@/lib/config';

export interface PublicAccount {
  id: string;
  username: string;
  acct: string;
  display_name: string;
  note: string;
  url: string;
  avatar: string;
  header: string;
  locked: boolean;
  bot: boolean;
  created_at: string;
  followers_count: number;
  following_count: number;
  statuses_count: number;
  fields: Array<{ name: string; value: string; verified_at: string | null }>;
}

export async function getPublicAccount(username: string): Promise<PublicAccount> {
  const config = await getConfig();
  const res = await fetch(
    `${config.server_url}/api/v1/accounts/lookup?acct=${encodeURIComponent(username)}`
  );
  if (res.status === 404) throw new Error('Profile not found');
  if (!res.ok) throw new Error('Failed to load profile');
  return res.json() as Promise<PublicAccount>;
}

export interface RegisterAccountInput {
  username: string;
  email: string;
  password: string;
  agreement: boolean;
  reason?: string;
  invite_code?: string;
}

export interface RegisterAccountResponse {
  account: { id: string; username: string };
  pending: boolean;
}

export async function registerAccount(
  input: RegisterAccountInput
): Promise<RegisterAccountResponse> {
  const config = await getConfig();
  const res = await fetch(`${config.server_url}/api/v1/accounts`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error ?? 'Registration failed');
  }
  return res.json() as Promise<RegisterAccountResponse>;
}
