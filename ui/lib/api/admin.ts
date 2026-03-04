import { authFetch } from './client';
import { getConfig } from '@/lib/config';

const adminPrefix = () => getConfig().then((c) => `${c.server_url}/monstera/api/v1/admin`);

export interface DashboardResponse {
  local_users_count: number;
  local_statuses_count: number;
  open_reports_count: number;
}

export async function getDashboard(): Promise<DashboardResponse> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/dashboard`);
  if (!res.ok) throw new Error('Failed to fetch dashboard');
  return res.json();
}

export interface AdminUser {
  id: string;
  account_id: string;
  email: string;
  role: string;
  username: string;
  suspended: boolean;
  silenced: boolean;
}

export interface UserListResponse {
  users: AdminUser[];
}

export async function getUsers(params?: { limit?: number; offset?: number }): Promise<UserListResponse> {
  const base = await adminPrefix();
  const search = new URLSearchParams();
  if (params?.limit != null) search.set('limit', String(params.limit));
  if (params?.offset != null) search.set('offset', String(params.offset));
  const q = search.toString();
  const url = q ? `${base}/users?${q}` : `${base}/users`;
  const res = await authFetch(url);
  if (!res.ok) throw new Error('Failed to fetch users');
  return res.json();
}

export async function getUser(id: string): Promise<AdminUser> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}`);
  if (!res.ok) throw new Error('Failed to fetch user');
  return res.json();
}

export async function suspendUser(id: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}/suspend`, { method: 'POST' });
  if (!res.ok) throw new Error('Failed to suspend user');
}

export async function unsuspendUser(id: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}/unsuspend`, { method: 'POST' });
  if (!res.ok) throw new Error('Failed to unsuspend user');
}

export async function silenceUser(id: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}/silence`, { method: 'POST' });
  if (!res.ok) throw new Error('Failed to silence user');
}

export async function unsilenceUser(id: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}/unsilence`, { method: 'POST' });
  if (!res.ok) throw new Error('Failed to unsilence user');
}

export async function setUserRole(id: string, role: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}/role`, {
    method: 'PUT',
    body: JSON.stringify({ role }),
  });
  if (!res.ok) throw new Error('Failed to set role');
}

export async function deleteUser(id: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error('Failed to delete user');
}

export interface PendingRegistration {
  user_id: string;
  account_id: string;
  email: string;
  username: string;
  registration_reason?: string;
}

export interface RegistrationsResponse {
  pending: PendingRegistration[];
}

export async function getRegistrations(): Promise<RegistrationsResponse> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/registrations`);
  if (!res.ok) throw new Error('Failed to fetch registrations');
  return res.json();
}

export async function approveRegistration(id: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/registrations/${encodeURIComponent(id)}/approve`, { method: 'POST' });
  if (!res.ok) throw new Error('Failed to approve');
}

export async function rejectRegistration(id: string, reason: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/registrations/${encodeURIComponent(id)}/reject`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  });
  if (!res.ok) throw new Error('Failed to reject');
}

export interface Invite {
  id: string;
  code: string;
  uses: number;
  max_uses?: number;
  expires_at?: string;
  created_at: string;
}

export interface InvitesResponse {
  invites: Invite[];
}

export async function getInvites(): Promise<InvitesResponse> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/invites`);
  if (!res.ok) throw new Error('Failed to fetch invites');
  return res.json();
}

export async function createInvite(body?: { max_uses?: number; expires_at?: string }): Promise<Invite> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/invites`, {
    method: 'POST',
    body: JSON.stringify(body ?? {}),
  });
  if (!res.ok) throw new Error('Failed to create invite');
  return res.json();
}

export async function deleteInvite(id: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/invites/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error('Failed to delete invite');
}

export interface Report {
  id: string;
  account_id: string;
  target_id: string;
  status_ids: string[];
  comment?: string;
  category: string;
  state: string;
  assigned_to_id?: string;
  action_taken?: string;
  created_at: string;
  resolved_at?: string;
}

export interface ReportsResponse {
  reports: Report[];
}

export async function getReports(params?: { state?: string; limit?: number; offset?: number }): Promise<ReportsResponse> {
  const base = await adminPrefix();
  const search = new URLSearchParams();
  if (params?.state != null) search.set('state', params.state);
  if (params?.limit != null) search.set('limit', String(params.limit));
  if (params?.offset != null) search.set('offset', String(params.offset));
  const q = search.toString();
  const url = q ? `${base}/reports?${q}` : `${base}/reports`;
  const res = await authFetch(url);
  if (!res.ok) throw new Error('Failed to fetch reports');
  return res.json();
}

export async function getReport(id: string): Promise<Report> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/reports/${encodeURIComponent(id)}`);
  if (!res.ok) throw new Error('Failed to fetch report');
  return res.json();
}

export async function assignReport(id: string, assigneeId: string | null): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/reports/${encodeURIComponent(id)}/assign`, {
    method: 'POST',
    body: JSON.stringify({ assignee_id: assigneeId }),
  });
  if (!res.ok) throw new Error('Failed to assign report');
}

export async function resolveReport(id: string, resolution: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/reports/${encodeURIComponent(id)}/resolve`, {
    method: 'POST',
    body: JSON.stringify({ resolution }),
  });
  if (!res.ok) throw new Error('Failed to resolve report');
}

export interface KnownInstance {
  id: string;
  domain: string;
  software?: string;
  software_version?: string;
  first_seen_at: string;
  last_seen_at: string;
  accounts_count: number;
}

export interface DomainBlock {
  id: string;
  domain: string;
  severity: string;
  reason?: string;
  created_at: string;
}

export async function getInstances(params?: { limit?: number; offset?: number }): Promise<{ instances: KnownInstance[] }> {
  const base = await adminPrefix();
  const search = new URLSearchParams();
  if (params?.limit != null) search.set('limit', String(params.limit));
  if (params?.offset != null) search.set('offset', String(params.offset));
  const q = search.toString();
  const url = q ? `${base}/federation/instances?${q}` : `${base}/federation/instances`;
  const res = await authFetch(url);
  if (!res.ok) throw new Error('Failed to fetch instances');
  return res.json();
}

export async function getDomainBlocks(): Promise<{ domain_blocks: DomainBlock[] }> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/federation/domain-blocks`);
  if (!res.ok) throw new Error('Failed to fetch domain blocks');
  return res.json();
}

export async function createDomainBlock(body: { domain: string; severity?: string; reason?: string }): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/federation/domain-blocks`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error('Failed to create domain block');
}

export async function deleteDomainBlock(domain: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/federation/domain-blocks/${encodeURIComponent(domain)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error('Failed to delete domain block');
}

export interface ServerFilter {
  id: string;
  phrase: string;
  scope: string;
  action: string;
  whole_word: boolean;
  created_at: string;
  updated_at: string;
}

export async function getFilters(): Promise<{ filters: ServerFilter[] }> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/content/filters`);
  if (!res.ok) throw new Error('Failed to fetch filters');
  return res.json();
}

export async function createFilter(body: { phrase: string; scope?: string; action?: string; whole_word?: boolean }): Promise<ServerFilter> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/content/filters`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error('Failed to create filter');
  return res.json();
}

export async function updateFilter(id: string, body: { phrase: string; scope?: string; action?: string; whole_word?: boolean }): Promise<ServerFilter> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/content/filters/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error('Failed to update filter');
  return res.json();
}

export async function deleteFilter(id: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/content/filters/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error('Failed to delete filter');
}

export async function getSettings(): Promise<{ settings: Record<string, string> }> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/settings`);
  if (!res.ok) throw new Error('Failed to fetch settings');
  return res.json();
}

export async function putSettings(settings: Record<string, string>): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/settings`, {
    method: 'PUT',
    body: JSON.stringify({ settings }),
  });
  if (!res.ok) throw new Error('Failed to update settings');
}
