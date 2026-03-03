'use client';

import { getUser, isAdminOrModerator } from '@/lib/api/user';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import type { User } from '@/lib/api/user';

export default function ServerSettingsPage() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getUser()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!user || !isAdminOrModerator(user)) {
      router.replace('/home');
    }
  }, [loading, user, router]);

  if (loading || !user || !isAdminOrModerator(user)) {
    return null;
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900">Server settings</h1>
      <p className="mt-2 text-gray-500">
        Server configuration will appear here.
      </p>
    </div>
  );
}
