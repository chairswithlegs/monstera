import { authFetch } from "./client";
import { getConfig } from "@/lib/config";


export interface ProfileField {
  name: string;
  value: string;
  verified_at: string | null;
}

export interface User {
  id: string;
  account_id: string;
  username: string;
  email: string;
  confirmed_at: string;
  role: string;
  default_privacy: string;
  default_sensitive: boolean;
  default_language: string;
  default_quote_policy: string;
  created_at: string;
  display_name: string | null;
  note: string | null;
  locked: boolean;
  bot: boolean;
  fields: ProfileField[] | null;
}

export function isAdmin(user: User): boolean {
  return user.role === 'admin';
}

export function isAdminOrModerator(user: User): boolean {
  return user.role === 'admin' || user.role === 'moderator';
}

export async function getUser() {
  const config = await getConfig();
  const response = await authFetch(`${config.server_url}/monstera/api/v1/user`);
  if (!response.ok) {
    throw new Error('Failed to verify credentials');
  }
  return response.json() as Promise<User>;
}

export async function patchProfile(data: {
  display_name?: string | null;
  note?: string | null;
  locked?: boolean;
  bot?: boolean;
  fields?: ProfileField[];
  default_quote_policy?: string | null;
}): Promise<User> {
  const config = await getConfig();
  const response = await authFetch(`${config.server_url}/monstera/api/v1/account/profile`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!response.ok) {
    throw new Error('Failed to update profile');
  }
  return response.json() as Promise<User>;
}

export async function patchPreferences(data: {
  default_privacy: string;
  default_sensitive: boolean;
  default_language: string;
  default_quote_policy: string;
}): Promise<User> {
  const config = await getConfig();
  const response = await authFetch(`${config.server_url}/monstera/api/v1/account/preferences`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!response.ok) {
    throw new Error('Failed to update preferences');
  }
  return response.json() as Promise<User>;
}

export async function patchEmail(data: { email: string }): Promise<User> {
  const config = await getConfig();
  const response = await authFetch(`${config.server_url}/monstera/api/v1/account/security/email`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!response.ok) {
    throw new Error('Failed to update email');
  }
  return response.json() as Promise<User>;
}

export async function patchPassword(data: {
  current_password: string;
  new_password: string;
}): Promise<void> {
  const config = await getConfig();
  const response = await authFetch(`${config.server_url}/monstera/api/v1/account/security/password`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!response.ok) {
    const text = await response.text().catch(() => '');
    throw new Error(response.status === 403 ? 'Current password is incorrect' : (text || 'Failed to change password'));
  }
}
