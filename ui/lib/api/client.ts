import { getAccessToken, refreshAccessToken } from '@/lib/auth/tokens';
import { logout } from '@/lib/auth/logout';

let isRefreshing = false;
let refreshQueue: Array<(token: string) => void> = [];

async function getValidToken(): Promise<string> {
  // If a refresh is already in flight, queue up rather than firing multiple refresh calls
  if (isRefreshing) {
    return new Promise((resolve) => refreshQueue.push(resolve));
  }

  isRefreshing = true;
  try {
    const newToken = await refreshAccessToken();
    refreshQueue.forEach((resolve) => resolve(newToken));
    return newToken;
  } finally {
    isRefreshing = false;
    refreshQueue = [];
  }
}

export async function authFetch(
  url: string,
  options: RequestInit = {}
): Promise<Response> {
  const token = getAccessToken();

  const response = await fetch(url, {
    ...options,
    headers: {
      ...options.headers,
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
  });

  if (response.status === 401) {
    try {
      const newToken = await getValidToken();
      return fetch(url, {
        ...options,
        headers: {
          ...options.headers,
          Authorization: `Bearer ${newToken}`,
          'Content-Type': 'application/json',
        },
      });
    } catch {
      logout();
      throw new Error('Session expired');
    }
  }

  return response;
}
