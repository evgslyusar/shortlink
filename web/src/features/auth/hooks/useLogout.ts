import { useMutation } from "@tanstack/react-query";

import { logout } from "../api";

export function useLogout() {
  return useMutation({
    mutationFn: () => logout(),
  });
}
