'use client';
import { useRouter } from 'next/navigation';
import { useEffect } from 'react';

export default function ModeratorInvitesPage() {
  const router = useRouter();
  useEffect(() => { router.replace('/moderator/registrations'); }, [router]);
  return null;
}
