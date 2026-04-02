import { authFetch } from './client';
import { getConfig } from '@/lib/config';
import { throwApiError } from '@/lib/api/errors';

const adminPrefix = () => getConfig().then((c) => `${c.server_url}/monstera/api/v1/admin`);
const moderatorPrefix = () => getConfig().then((c) => `${c.server_url}/monstera/api/v1/moderator`);

export interface DashboardResponse {
  local_users_count: number;
  local_statuses_count: number;
  open_reports_count: number;
}

export async function getDashboard(): Promise<DashboardResponse> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/dashboard`);
  if (!res.ok) await throwApiError(res);
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
  const base = await moderatorPrefix();
  const search = new URLSearchParams();
  if (params?.limit != null) search.set('limit', String(params.limit));
  if (params?.offset != null) search.set('offset', String(params.offset));
  const q = search.toString();
  const url = q ? `${base}/users?${q}` : `${base}/users`;
  const res = await authFetch(url);
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function getUser(id: string): Promise<AdminUser> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}`);
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function suspendUser(id: string): Promise<void> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}/suspend`, { method: 'POST' });
  if (!res.ok) await throwApiError(res);
}

export async function unsuspendUser(id: string): Promise<void> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}/unsuspend`, { method: 'POST' });
  if (!res.ok) await throwApiError(res);
}

export async function silenceUser(id: string): Promise<void> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}/silence`, { method: 'POST' });
  if (!res.ok) await throwApiError(res);
}

export async function unsilenceUser(id: string): Promise<void> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}/unsilence`, { method: 'POST' });
  if (!res.ok) await throwApiError(res);
}

export async function setUserRole(id: string, role: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}/role`, {
    method: 'PUT',
    body: JSON.stringify({ role }),
  });
  if (!res.ok) await throwApiError(res);
}

export async function deleteUser(id: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/users/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) await throwApiError(res);
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
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/registrations`);
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function approveRegistration(id: string): Promise<void> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/registrations/${encodeURIComponent(id)}/approve`, { method: 'POST' });
  if (!res.ok) await throwApiError(res);
}

export async function rejectRegistration(id: string, reason: string): Promise<void> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/registrations/${encodeURIComponent(id)}/reject`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  });
  if (!res.ok) await throwApiError(res);
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
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/invites`);
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function createInvite(body?: { max_uses?: number; expires_at?: string }): Promise<Invite> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/invites`, {
    method: 'POST',
    body: JSON.stringify(body ?? {}),
  });
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function deleteInvite(id: string): Promise<void> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/invites/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) await throwApiError(res);
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
  const base = await moderatorPrefix();
  const search = new URLSearchParams();
  if (params?.state != null) search.set('state', params.state);
  if (params?.limit != null) search.set('limit', String(params.limit));
  if (params?.offset != null) search.set('offset', String(params.offset));
  const q = search.toString();
  const url = q ? `${base}/reports?${q}` : `${base}/reports`;
  const res = await authFetch(url);
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function getReport(id: string): Promise<Report> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/reports/${encodeURIComponent(id)}`);
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function assignReport(id: string, assigneeId: string | null): Promise<void> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/reports/${encodeURIComponent(id)}/assign`, {
    method: 'POST',
    body: JSON.stringify({ assignee_id: assigneeId }),
  });
  if (!res.ok) await throwApiError(res);
}

export async function resolveReport(id: string, resolution: string): Promise<void> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/reports/${encodeURIComponent(id)}/resolve`, {
    method: 'POST',
    body: JSON.stringify({ resolution }),
  });
  if (!res.ok) await throwApiError(res);
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
  const base = await moderatorPrefix();
  const search = new URLSearchParams();
  if (params?.limit != null) search.set('limit', String(params.limit));
  if (params?.offset != null) search.set('offset', String(params.offset));
  const q = search.toString();
  const url = q ? `${base}/federation/instances?${q}` : `${base}/federation/instances`;
  const res = await authFetch(url);
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function getDomainBlocks(): Promise<{ domain_blocks: DomainBlock[] }> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/federation/domain-blocks`);
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function createDomainBlock(body: { domain: string; severity?: string; reason?: string }): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/federation/domain-blocks`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
  if (!res.ok) await throwApiError(res);
}

export async function deleteDomainBlock(domain: string): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/federation/domain-blocks/${encodeURIComponent(domain)}`, { method: 'DELETE' });
  if (!res.ok) await throwApiError(res);
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
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/content/filters`);
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function createFilter(body: { phrase: string; scope?: string; action?: string; whole_word?: boolean }): Promise<ServerFilter> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/content/filters`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function updateFilter(id: string, body: { phrase: string; scope?: string; action?: string; whole_word?: boolean }): Promise<ServerFilter> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/content/filters/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  });
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function deleteFilter(id: string): Promise<void> {
  const base = await moderatorPrefix();
  const res = await authFetch(`${base}/content/filters/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok) await throwApiError(res);
}

export interface AdminSettings {
  registration_mode: string; // "open" | "approval" | "invite" | "closed"
  invite_max_uses?: number | null;
  invite_expires_in_days?: number | null;
  server_name?: string | null;
  server_description?: string | null;
  server_rules?: string[];
}

export async function getSettings(): Promise<AdminSettings> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/settings`);
  if (!res.ok) await throwApiError(res);
  return res.json();
}

export async function putSettings(settings: AdminSettings): Promise<void> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/settings`, {
    method: 'PUT',
    body: JSON.stringify(settings),
  });
  if (!res.ok) await throwApiError(res);
}

export interface AdminMetricsResponse {
  local_accounts: number;
  remote_accounts: number;
  local_statuses: number;
  remote_statuses: number;
  known_instances: number;
  open_reports: number;
  delivery_dlq_depth: number;
  fanout_dlq_depth: number;
}

export async function getAdminMetrics(): Promise<AdminMetricsResponse> {
  const base = await adminPrefix();
  const res = await authFetch(`${base}/metrics`);
  if (!res.ok) await throwApiError(res);
  return res.json();
}
