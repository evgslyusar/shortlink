import { useEffect } from "react";
import { RouterProvider } from "react-router-dom";

import { Providers } from "@/app/providers";
import { router } from "@/app/router";
import { setOnUnauthorized } from "@/api/client";
import { useAuth, useAuthInit } from "@/features/auth";

export function App() {
  const clearAuth = useAuth((s) => s.clearAuth);

  useEffect(() => {
    setOnUnauthorized(() => {
      clearAuth();
      router.navigate("/login");
    });
  }, [clearAuth]);

  useAuthInit();

  return (
    <Providers>
      <RouterProvider router={router} />
    </Providers>
  );
}
