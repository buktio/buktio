"use client";

import * as React from "react";
import type { Features, User } from "@/lib/api";

interface Session {
  user: User;
  features: Features;
}

const SessionContext = React.createContext<Session | null>(null);

export function UserProvider({
  user,
  features,
  children,
}: {
  user: User;
  features: Features;
  children: React.ReactNode;
}) {
  const value = React.useMemo<Session>(() => ({ user, features }), [user, features]);
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

/** Access the current authenticated user. Guaranteed non-null inside the dashboard. */
export function useUser(): User {
  return useSession().user;
}

/** Access the current Pro feature flags. Guaranteed non-null inside the dashboard. */
export function useFeatures(): Features {
  return useSession().features;
}

/** Access the full session (user + features). Non-null inside the dashboard. */
export function useSession(): Session {
  const session = React.useContext(SessionContext);
  if (!session) {
    throw new Error("useSession must be used within the dashboard UserProvider");
  }
  return session;
}
