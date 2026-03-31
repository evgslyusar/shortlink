import { Navigate } from "react-router-dom";
import type { ReactNode } from "react";

import { useAuth } from "@/features/auth";

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}
