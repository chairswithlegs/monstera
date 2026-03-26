import { getConfig } from '@/lib/config';
import { ApiResponseError } from '@/lib/api/errors';

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
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string; code?: string; params?: Record<string, string> };
    throw new ApiResponseError(body);
  }
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
    const body = await res.json().catch(() => ({})) as { error?: string; code?: string; params?: Record<string, string> };
    throw new ApiResponseError(body);
  }
  return res.json() as Promise<RegisterAccountResponse>;
}
