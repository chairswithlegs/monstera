import { authFetch } from "./client";
import { getConfig } from "@/lib/config";
import { throwApiError } from "@/lib/api/errors";

export interface AuthorizedApp {
  id: string;
  name: string;
  website: string | null;
  redirect_uris: string;
  scopes: string[];
  created_at: string;
}

export interface VerifiedAppCredentials {
  id: string;
  name: string;
  website: string | null;
  vapid_key: string;
}

export async function listAuthorizedApps(): Promise<AuthorizedApp[]> {
  const config = await getConfig();
  const response = await authFetch(`${config.server_url}/api/v1/oauth/authorized_applications`);
  if (!response.ok) {
    await throwApiError(response);
  }
  return response.json() as Promise<AuthorizedApp[]>;
}

export async function revokeAuthorizedApp(appId: string): Promise<void> {
  const config = await getConfig();
  const response = await authFetch(
    `${config.server_url}/api/v1/oauth/authorized_applications/${encodeURIComponent(appId)}`,
    { method: "DELETE" },
  );
  if (!response.ok) {
    await throwApiError(response);
  }
}

export async function verifyAppCredentials(): Promise<VerifiedAppCredentials> {
  const config = await getConfig();
  const response = await authFetch(`${config.server_url}/api/v1/apps/verify_credentials`);
  if (!response.ok) {
    await throwApiError(response);
  }
  return response.json() as Promise<VerifiedAppCredentials>;
}
