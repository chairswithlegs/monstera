'use client';

import { useRouter } from 'next/navigation';
import { useEffect } from 'react';

export default function ModeratorIndexPage() {
  const router = useRouter();
  useEffect(() => {
    router.replace('/moderator/dashboard');
  }, [router]);
  return null;
}
