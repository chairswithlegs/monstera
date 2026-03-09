import { getConfig } from '@/lib/config';

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
