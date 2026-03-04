'use client';

import type { User } from '@/lib/api/user';
import { createContext, useContext } from 'react';

const ModeratorUserContext = createContext<User | null>(null);

export function ModeratorUserProvider({
  user,
  children,
}: {
  user: User | null;
  children: React.ReactNode;
}) {
  return (
    <ModeratorUserContext.Provider value={user}>
      {children}
    </ModeratorUserContext.Provider>
  );
}

export function useModeratorUser(): User | null {
  return useContext(ModeratorUserContext);
}
