import { useEffect, useRef } from "react";

import { refresh } from "../api";

import { useAuth } from "./useAuth";

/**
 * Attempts to restore auth state on mount by calling POST /auth/refresh.
 * If a valid refresh_token cookie exists, the backend returns new tokens
 * and the Zustand store is populated. Otherwise, the user stays logged out.
 */
export function useAuthInit() {
  const setAuth = useAuth((s) => s.setAuth);
  const isAuthenticated = useAuth((s) => s.isAuthenticated);
  const attempted = useRef(false);

  useEffect(() => {
    if (attempted.current || isAuthenticated) return;
    attempted.current = true;

    refresh()
      .then(() => {
        // Refresh succeeded — cookies are set, but we don't have user info
        // from the refresh endpoint. Mark as authenticated with minimal info.
        // A future /auth/me endpoint could provide full user details.
        setAuth({ id: "", email: "" });
      })
      .catch(() => {
        // No valid refresh token — user stays logged out.
      });
  }, [setAuth, isAuthenticated]);
}
