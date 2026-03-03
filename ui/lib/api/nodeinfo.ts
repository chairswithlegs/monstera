import { getConfig } from "@/lib/config";

export interface NodeInfoResponse {
  version: string;
  software: NodeInfoSoftware;
  protocols: string[];
  usage: NodeInfoUsage;
  openRegistrations: boolean;
  metadata: Record<string, unknown>;
}

export interface NodeInfoSoftware {
  name: string;
  version: string;
}

export interface NodeInfoUsage {
  users: NodeInfoUsers;
  localPosts: number;
}

export interface NodeInfoUsers {
  total: number;
}

export async function getNodeInfo(): Promise<NodeInfoResponse> {
  const config = await getConfig();
  const response = await fetch(`${config.server_url}/nodeinfo/2.0`);
  if (!response.ok) {
    throw new Error("Failed to load server information");
  }
  return response.json() as Promise<NodeInfoResponse>;
}
