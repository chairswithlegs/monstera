import { authFetch } from "./client";
import { getMonsteraServerUrl } from "@/lib/config";

const MONSTERA_SERVER = getMonsteraServerUrl();

export interface User {
  id: string;
  account_id: string;
  email: string;
  confirmed_at: string;
  role: string;
  default_privacy: string;
  default_sensitive: boolean;
  default_language: string;
  created_at: string;
}

export async function getUser() {
  const response = await authFetch(`${MONSTERA_SERVER}/monstera/api/v1/user`);
  if (!response.ok) {
    throw new Error('Failed to verify credentials');
  }
  return response.json() as Promise<User>;
}